package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

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
	mbzIdCmd.Flags().BoolVar(&mbzidNoScan, "no-scan", false, `don't re-scan afterwards.
WARNING: Your database will be in an inconsistent state unless a full-rescan is completed.`)
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

func migrateUserPlaylists(ctx context.Context, ds model.DataStore, user model.User, ndIdToMbz map[string]*model.MediaFile) error {
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

		for i, track := range newPlaylist.Tracks {
			if newTrack, found := ndIdToMbz[track.MediaFileID]; found {
				newPlaylist.Tracks[i].MediaFileID = newTrack.ID
				newPlaylist.Tracks[i].MediaFile.ID = newTrack.ID
			}
		}

		if err2 = repo.Put(newPlaylist); err2 != nil {
			return err2
		}
	}
	return nil
}

func migrateUserPlayQueue(ctx context.Context, ds model.DataStore, user model.User, ndIdToMbz map[string]*model.MediaFile) error {
	repo := ds.PlayQueue(request.WithUser(ctx, user))
	playQueue, err := repo.Retrieve(user.ID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}

		return err
	}

	if newTrack, found := ndIdToMbz[playQueue.Current]; found {
		playQueue.Current = newTrack.ID
	}

	for i, item := range playQueue.Items {
		if newTrack, found := ndIdToMbz[item.ID]; found {
			playQueue.Items[i].ID = newTrack.ID
		}
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

	oldToNewMF := make(map[string]*model.MediaFile, len(mediaFiles))

	newMediaFiles := make(map[string]*model.MediaFile, len(mediaFiles))
	newArtists := make(map[string]*model.Artist)
	newAlbums := make(map[string]*model.Album)

	oldMediaFiles := map[string]bool{}
	oldArtists := map[string]bool{}
	oldAlbums := map[string]bool{}

	for _, mf := range mediaFiles {
		// Don't touch partial files. The final rescan should take care of them.
		if mf.MbzTrackID == "" || mf.MbzAlbumID == "" || mf.MbzArtistID == "" || mf.MbzAlbumArtistID == "" {
			continue
		}

		oldMediaFiles[mf.ID] = true
		oldArtists[mf.ArtistID] = true
		oldArtists[mf.AlbumArtistID] = true
		oldAlbums[mf.AlbumID] = true

		newID := fmt.Sprintf("%v-%v", mf.MbzAlbumID, mf.MbzTrackID)

		if newMediaFile, ok := newMediaFiles[newID]; !ok {
			newMediaFile = &model.MediaFile{}
			*newMediaFile = mf

			newMediaFile.ID = newID
			newMediaFile.AlbumID = mf.MbzAlbumID
			newMediaFile.ArtistID = mf.MbzArtistID
			newMediaFile.AlbumArtistID = mf.MbzAlbumArtistID
			newMediaFiles[newID] = newMediaFile

			oldToNewMF[mf.ID] = newMediaFile
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
				ArtistID:         mf.MbzArtistID,
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
			newAlbum.ArtistID = tmp.ArtistID
			newAlbum.AlbumArtistID = tmp.AlbumArtistID
			newAlbum.MbzAlbumID = tmp.MbzAlbumID
			newAlbum.MbzAlbumArtistID = tmp.MbzAlbumArtistID
			newAlbum.AllArtistIDs = "" // Nuking this, the rescan will fix it
		}
	}

	log.Info("Pass 2: Add new artists")
	for _, artist := range newArtists {
		if err = artistRepo.Put(artist); err != nil {
			return err
		}
	}

	log.Info("Pass 3: Add new albums")
	for _, album := range newAlbums {
		if err = albumRepo.Put(album); err != nil {
			return err
		}
	}

	log.Info("Pass 4: Add new tracks")
	for _, mf := range newMediaFiles {
		if err = mfRepo.Put(mf); err != nil {
			return err
		}
	}

	// Playlists and Play queues require a user in the context
	users, err := userRepo.GetAll()
	if err != nil {
		return err
	}

	log.Info("Pass 5: Update playlist references")
	for _, user := range users {
		if err = migrateUserPlaylists(ctx, ds, user, oldToNewMF); err != nil {
			return err
		}
	}

	log.Info("Pass 6: Update play queue references")
	for _, user := range users {
		if err = migrateUserPlayQueue(ctx, ds, user, oldToNewMF); err != nil {
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
	//return ds.GC(ctx, "")
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
