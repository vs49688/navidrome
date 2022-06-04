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

type deleteManyable interface {
	DeleteMany(ids ...string) error
}

func deleteManyIDs(repo deleteManyable, ids map[string]bool) error {
	s := make([]string, 0, len(ids))
	for id := range ids {
		s = append(s, id)
	}

	return utils.RangeByChunks(s, 100, func(s []string) error {
		return repo.DeleteMany(s...)
	})
}

func migrateUserPlaylists(ctx context.Context, ds model.DataStore, user model.User, ndIdToMbzId map[string]string) error {
	var err error

	repo := ds.Playlist(request.WithUser(ctx, user))
	playlists, err := repo.GetAll()
	if err != nil {
		return err
	}

	for _, playlist := range playlists {
		newPlaylist, err2 := repo.GetWithTracks(playlist.ID)
		if err2 != nil {
			return err2
		}

		for i := range newPlaylist.Tracks {
			newPlaylist.Tracks[i].MediaFileID = ndIdToMbzId[newPlaylist.Tracks[i].MediaFileID]
			newPlaylist.Tracks[i].MediaFile.ID = newPlaylist.Tracks[i].MediaFileID
		}

		if err2 = repo.Put(newPlaylist); err2 != nil {
			return err2
		}
	}
	return nil
}

func migrateUserPlayQueue(ctx context.Context, ds model.DataStore, user model.User, ndIdToMbzId map[string]string) error {
	repo := ds.PlayQueue(request.WithUser(ctx, user))
	playQueue, err := repo.Retrieve(user.ID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}

		return err
	}

	playQueue.Current = ndIdToMbzId[playQueue.Current]

	for i := range playQueue.Items {
		playQueue.Items[i].ID = xxx[playQueue.Items[i].ID]
	}

	return repo.Store(playQueue)
}

// Migrate all the database entities to use MusicBrainz IDs.
// Uses the Mbz* fields in model.MediaFile to define the relationships, ignoring
// the Navidrome ones.
func migrateEverything(ctx context.Context, ds model.DataStore) error {
	artistRepo := ds.Artist(ctx)
	albumRepo := ds.Album(ctx)
	mfRepo := ds.MediaFile(ctx)
	userRepo := ds.User(ctx)

	log.Info("Pass 1: Rebuild hierarchy")

	mediaFiles, err := mfRepo.GetAll()
	if err != nil {
		return err
	}

	mediaFileIdMap := make(map[string]*model.MediaFile, len(mediaFiles))
	newMediaFiles := make(map[string]*model.MediaFile, len(mediaFiles))
	newArtists := make(map[string]*model.Artist)
	newAlbums := make(map[string]*model.Album)

	oldMediaFiles := map[string]bool{}
	oldArtists := map[string]bool{}
	oldAlbums := map[string]bool{}

	for i, mf := range mediaFiles {
		mediaFileIdMap[mf.ID] = &mediaFiles[i]

		// Don't touch partial files. The final rescan should take care of them.
		if mf.MbzTrackID == "" || mf.MbzAlbumID == "" || mf.MbzArtistID == "" || mf.MbzAlbumArtistID == "" {
			continue
		}

		oldMediaFiles[mf.ID] = true
		oldArtists[mf.ArtistID] = true
		oldArtists[mf.AlbumArtistID] = true
		oldAlbums[mf.AlbumID] = true

		if newMediaFile, ok := newMediaFiles[mf.MbzTrackID]; !ok {
			newMediaFile = &model.MediaFile{}
			*newMediaFile = mf

			newMediaFile.ID = mf.MbzTrackID
			newMediaFile.AlbumID = mf.MbzAlbumID
			newMediaFile.ArtistID = mf.MbzArtistID
			newMediaFile.AlbumArtistID = mf.MbzAlbumArtistID
			newMediaFiles[mf.MbzTrackID] = newMediaFile
		}

		if _, ok := newArtists[mf.MbzArtistID]; !ok {
			newArtists[mf.MbzArtistID] = &model.Artist{ID: mf.MbzArtistID, MbzArtistID: mf.MbzArtistID}
		}

		if _, ok := newArtists[mf.MbzAlbumArtistID]; !ok {
			newArtists[mf.MbzAlbumArtistID] = &model.Artist{ID: mf.MbzAlbumArtistID, MbzArtistID: mf.MbzAlbumArtistID}
		}

		if _, ok := newAlbums[mf.MbzAlbumID]; !ok {
			newAlbums[mf.MbzAlbumID] = &model.Album{
				ID:               mf.MbzAlbumID,
				AlbumArtistID:    mf.MbzAlbumArtistID,
				MbzAlbumID:       mf.MbzAlbumID,
				MbzAlbumArtistID: mf.MbzAlbumArtistID,
			}
		}

	}

	artists, err := artistRepo.GetAll()
	if err != nil {
		return err
	}

	for _, artist := range artists {
		if newArtist, ok := newArtists[artist.MbzArtistID]; ok {
			tmp := *newArtist
			*newArtist = artist
			newArtist.ID = tmp.ID
			newArtist.MbzArtistID = tmp.MbzArtistID
		}

		oldArtists[artist.ID] = true
	}

	albums, err := albumRepo.GetAll()
	if err != nil {
		return err
	}

	for _, album := range albums {
		if newAlbum, ok := newAlbums[album.MbzAlbumID]; ok {
			tmp := *newAlbum
			*newAlbum = album
			newAlbum.ID = tmp.ID
			newAlbum.AlbumArtistID = tmp.AlbumArtistID
			newAlbum.MbzAlbumID = tmp.MbzAlbumID
			newAlbum.MbzAlbumArtistID = tmp.MbzAlbumArtistID
		}

		oldAlbums[album.ID] = true
	}

	log.Info("Pass 2: Add new artists")
	// TODO: add new artists

	log.Info("Pass 3: Add new albums")
	// TODO: add new albums

	log.Info("Pass 4: Add new tracks")
	// TODO: add new mediafiles

	// Playlists and Play queues require a user in the context
	users, err := userRepo.GetAll()
	if err != nil {
		return err
	}

	log.Info("Pass 5: Update playlist references")
	for _, user := range users {
		if err = migrateUserPlaylists(ctx, ds, user, nil); err != nil {
			return err
		}
	}

	log.Info("Pass 6: Update play queue references")
	for _, user := range users {
		if err = migrateUserPlayQueue(ctx, ds, user, nil); err != nil {
			return err
		}
	}

	log.Info("Pass 7: Cleanup leftover tracks")
	if err = deleteManyIDs(mfRepo, oldMediaFiles); err != nil {
		return err
	}

	log.Info("Pass 8: Cleanup leftover albums")
	if err = deleteManyIDs(albumRepo, oldAlbums); err != nil {
		return err
	}

	log.Info("Pass 9: Cleanup leftover artists")
	if err = deleteManyIDs(artistRepo, oldArtists); err != nil {
		return err
	}

	return nil
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

		if err := migrateEverything(ctx, tx); err != nil {
			return err
		}

		//log.Info("Migrating artists...")
		//if err := migrateArtists(ctx, tx); err != nil {
		//	return err
		//}
		//
		//log.Info("Migrating albums...")
		//if err := migrateAlbums(ctx, tx); err != nil {
		//	return err
		//}

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
