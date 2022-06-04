package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/persistence"
	"github.com/navidrome/navidrome/utils"
	"github.com/spf13/cobra"
)

var mbzidNoScan bool
var mbzidNoConfirm bool

var mbzIdCmd = &cobra.Command{
	Use:   "use_mbzid",
	Short: "Use MusicBrainz IDs",
	Long:  "Convert Navidrome's database to use MusicBrainz IDs",
	Run: func(cmd *cobra.Command, args []string) {
		db.EnsureLatestVersion()
		if err := convertToMbzIDs(cmd.Context()); err != nil {
			log.Error("Error handling MusicBrainz cataloging. Aborting", err)
			os.Exit(1)
			return
		}
	},
}

func init() {
	mbzIdCmd.Flags().BoolVar(&mbzidNoScan, "no-scan", false, "don't re-scan afterwards")
	mbzIdCmd.Flags().BoolVar(&mbzidNoConfirm, "no-confirm", false, "don't ask for confirmation")
	rootCmd.AddCommand(mbzIdCmd)
}

func warnMbzMigration(dur time.Duration) bool {
	log.Warn("About to convert database to use MusicBrainz metadata. This CANNOT be undone.")
	log.Warn(fmt.Sprintf("If this isn't intentional, press ^C NOW. Will begin in %s...", dur))

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)

	defer signal.Stop(sc)

	select {
	case <-sc:
		return false
	case <-time.After(dur):
		return true
	}
}

type mbidMap map[string][]string

func (m mbidMap) maybeGet(s string, idx uint) string {
	if val, ok := m[s]; ok {
		return val[idx]
	}

	return s
}

func migrateArtists(ctx context.Context, ds model.DataStore) error {
	log.Info("Pass 1: Create new artists using MusicBrainz IDs")

	artistRepo := ds.Artist(ctx)

	artists, err := artistRepo.GetAll()
	if err != nil {
		return err
	}

	mbmap := make(mbidMap, len(artists))
	toRemove := make([]string, 0, len(artists))

	for _, a := range artists {
		if a.MbzArtistID == "" {
			continue
		}

		if _, err := uuid.Parse(a.MbzArtistID); err != nil {
			log.Warn(fmt.Sprintf("Ignoring invalid artist MBID %s", a.MbzArtistID))
			continue
		}

		toRemove = append(toRemove, a.ID)
		mbmap[a.ID] = append(mbmap[a.ID], a.MbzArtistID)

		oldID := a.ID
		a.ID = a.MbzArtistID
		if err = artistRepo.Put(&a); err != nil {
			return err
		}

		if err = artistRepo.MoveAnnotation(oldID, a.ID); err != nil {
			return err
		}
	}

	log.Info("Pass 2: Update album artist references")
	albumRepo := ds.Album(ctx)
	albums, err := albumRepo.GetAll()
	if err != nil {
		return err
	}

	for _, album := range albums {
		album.ArtistID = mbmap.maybeGet(album.ArtistID, 0)
		album.AlbumArtistID = mbmap.maybeGet(album.AlbumArtistID, 0)

		allArtistIDs := strings.Split(album.AllArtistIDs, " ")
		newArtists := make([]string, 0, len(allArtistIDs))

		for _, a := range allArtistIDs {
			newArtists = append(newArtists, strings.TrimSpace(mbmap.maybeGet(a, 0)))
		}
		album.AllArtistIDs = utils.SanitizeStrings(newArtists...)

		if err = albumRepo.Put(&album); err != nil {
			return err
		}
	}

	log.Info("Pass 3: Update track artist references")
	mfRepo := ds.MediaFile(ctx)
	mediaFiles, err := mfRepo.GetAll()
	if err != nil {
		return err
	}

	for _, mf := range mediaFiles {
		mf.ArtistID = mbmap.maybeGet(mf.ArtistID, 0)
		mf.AlbumArtistID = mbmap.maybeGet(mf.AlbumArtistID, 0)

		if err = mfRepo.Put(&mf); err != nil {
			return err
		}
	}

	log.Info(fmt.Sprintf("Pass 4: Cleanup %v leftover artists", len(toRemove)))
	return utils.RangeByChunks(toRemove, 100, func(s []string) error {
		return artistRepo.DeleteMany(s...)
	})
}

func migrateAlbums(ctx context.Context, ds model.DataStore) error {
	albumRepo := ds.Album(ctx)
	albums, err := albumRepo.GetAll()
	if err != nil {
		return err
	}

	mfRepo := ds.MediaFile(ctx)
	mediaFiles, err := mfRepo.GetAll()
	if err != nil {
		return err
	}

	log.Info("Pass 1: Build album/track indexes")
	mediaFileIdMap := make(map[string]*model.MediaFile, len(mediaFiles))
	albumIdMap := make(map[string]model.Album, len(albums))
	mbToNdAlbum := make(map[string]*model.Album, len(albums))
	toRemove := make([]string, 0, len(albums))

	for _, a := range albums {
		albumIdMap[a.ID] = a
		toRemove = append(toRemove, a.ID)
	}

	for _, mf := range mediaFiles {
		mediaFileIdMap[mf.ID] = &mf

		if mf.MbzAlbumID == "" {
			continue
		}

		if _, err := uuid.Parse(mf.MbzAlbumID); err != nil {
			log.Warn(fmt.Sprintf("Ignoring invalid track album MBID %s", mf.MbzAlbumID))
			continue
		}

		// Copy the existing album and update its IDs
		newAlbum := &model.Album{}
		*newAlbum = albumIdMap[mf.AlbumID]
		newAlbum.ID = mf.MbzAlbumID
		newAlbum.MbzAlbumID = mf.MbzAlbumID

		mbToNdAlbum[mf.MbzAlbumID] = newAlbum
	}

	log.Info("Pass 2: Create new albums with MusicBrainz IDs")
	for _, newAlbum := range mbToNdAlbum {
		if err := albumRepo.Put(newAlbum); err != nil {
			return err
		}

		// TODO: copy annotation
	}

	log.Info("Pass 3: Update track album references")

	for _, mf := range mediaFiles {
		if mf.MbzAlbumID == "" {
			continue // Have already reported this above
		}

		mf.AlbumID = mf.MbzAlbumID
		if err := mfRepo.Put(&mf); err != nil {
			return err
		}
	}

	log.Info(fmt.Sprintf("Pass 3: Cleanup %v leftover albums", len(toRemove)))
	return utils.RangeByChunks(toRemove, 100, func(s []string) error {
		return albumRepo.DeleteMany(s...)
	})
}

func migrateMediaFiles(ctx context.Context, ds model.DataStore) error {
	log.Info("Pass 1: Create new tracks using MusicBrainz IDs")

	mfRepo := ds.MediaFile(ctx)
	mediaFiles, err := mfRepo.GetAll()
	if err != nil {
		return err
	}

	mbmap := make(mbidMap, len(mediaFiles))
	toRemove := make([]string, 0, len(mediaFiles))

	for _, mf := range mediaFiles {
		if mf.MbzTrackID == "" || mf.MbzAlbumID == "" {
			continue
		}

		if _, err := uuid.Parse(mf.MbzTrackID); err != nil {
			log.Warn(fmt.Sprintf("Ignoring invalid track MBID %s", mf.MbzTrackID))
			continue
		}

		if _, err := uuid.Parse(mf.MbzAlbumID); err != nil {
			log.Warn(fmt.Sprintf("Ignoring invalid album MBID %s", mf.MbzAlbumID))
			continue
		}

		toRemove = append(toRemove, mf.ID)
		// The same track can belong to multiple albums, so munge a key based on (album, track) ids
		newID := fmt.Sprintf("%v-%v", mf.MbzAlbumID, mf.MbzTrackID)
		mbmap[mf.ID] = append(mbmap[mf.ID], newID)

		if len(mbmap[mf.ID]) > 1 {
			fmt.Println("xxx")
		}

		oldID := mf.ID
		mf.ID = newID
		if err := mfRepo.Put(&mf); err != nil {
			return err
		}

		if err := mfRepo.MoveAnnotation(oldID, mf.ID); err != nil {
			return err
		}
	}

	// Playlists and Play queues require a user in the context
	userRepo := ds.User(ctx)
	users, err := userRepo.GetAll()
	if err != nil {
		return err
	}

	log.Info("Pass 2: Update playlist track references")
	for _, user := range users {
		playlistRepo := ds.Playlist(request.WithUser(ctx, user))
		playlists, err := playlistRepo.GetAll()
		if err != nil {
			return err
		}

		for _, playlist := range playlists {
			pl, err := playlistRepo.GetWithTracks(playlist.ID)
			if err != nil {
				return err
			}

			for i := range pl.Tracks {
				pl.Tracks[i].MediaFileID = mbmap.maybeGet(pl.Tracks[i].MediaFileID, 0)
				pl.Tracks[i].MediaFile.ID = pl.Tracks[i].MediaFileID
			}

			if err := playlistRepo.Put(pl); err != nil {
				return err
			}
		}
	}

	log.Info("Pass 3: Update play queue track references")
	for _, user := range users {
		playQueueRepo := ds.PlayQueue(request.WithUser(ctx, user))
		playQueue, err := playQueueRepo.Retrieve(user.ID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue
			}
			return err
		}

		playQueue.Current = mbmap.maybeGet(playQueue.Current, 0)

		for i := range playQueue.Items {
			playQueue.Items[i].ID = mbmap.maybeGet(playQueue.Items[i].ID, 0)
		}

		if err := playQueueRepo.Store(playQueue); err != nil {
			return err
		}

	}

	log.Info(fmt.Sprintf("Pass 4: Cleanup %v leftover tracks", len(toRemove)))
	return utils.RangeByChunks(toRemove, 100, func(s []string) error {
		return mfRepo.DeleteMany(s...)
	})
}

func convertToMbzIDs(ctx context.Context) error {
	var err error

	ds := persistence.New(db.Db())

	alreadyDone := false

	err = ds.WithTx(func(tx model.DataStore) error {
		props := tx.Property(ctx)

		useMbzIDs, err := props.DefaultGetBool(model.PropUsingMbzIDs, false)
		if err != nil {
			return err
		}

		// Nothing to do
		if useMbzIDs {
			alreadyDone = true
			return nil
		}

		if !mbzidNoConfirm && !warnMbzMigration(10*time.Second) {
			return errors.New("user aborted")
		}

		log.Info("Migrating artists...")
		if err := migrateArtists(ctx, tx); err != nil {
			return err
		}

		log.Info("Migrating albums...")
		if err := migrateAlbums(ctx, tx); err != nil {
			return err
		}

		//log.Info("Migrating tracks...")
		//if err = migrateMediaFiles(ctx, tx); err != nil {
		//	return err
		//}

		if err = props.Put(model.PropUsingMbzIDs, "true"); err != nil {
			return err
		}

		return props.DeletePrefixed(model.PropLastScan)
	})

	if err != nil {
		return err
	}

	if alreadyDone {
		log.Info("Migration already done.")
		return nil
	}

	if mbzidNoScan {
		log.Info("Skipping post-migration scan by request.")
		return nil
	}

	fullRescan = true
	runScanner()
	return nil
}
