package cmd

import (
	"context"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
	"github.com/stretchr/testify/assert"
	"testing"
)

// An actual example from my library, simplified.
// Two copies of "The Black Halo", one from Brazil, one from Japan.
// Both copies have the same recording as the first track, but different tracks
// towards the end of the album.
func createKamelotExample() model.DataStore {
	ctx := context.Background()
	ds := tests.MockDataStore{}

	kamelot := &model.Artist{
		ID:          "6194d02002d6ed297500ec7c04ad07d8",
		Name:        "Kamelot",
		MbzArtistID: "2449300a-6ca7-45da-8102-22789d256475",
	}

	_ = ds.Artist(ctx).Put(kamelot)

	theBlackHalo := &model.Album{
		ID:               "746632dea6742ff1234b62ba3505ec3c",
		Name:             "The Black Halo",
		ArtistID:         "6194d02002d6ed297500ec7c04ad07d8",
		MbzAlbumID:       "ec77f421-084d-4c0e-9966-257a549823d3",
		MbzAlbumArtistID: "2449300a-6ca7-45da-8102-22789d256475",
		AlbumArtistID:    "6194d02002d6ed297500ec7c04ad07d8",
		AllArtistIDs:     "6194d02002d6ed297500ec7c04ad07d8",
	}

	_ = ds.Album(ctx).Put(theBlackHalo)

	brazilTracks := []*model.MediaFile{
		{
			ID:               "1b28d36bd21837c24d5ad766a21a345d",
			Title:            "March of Mephisto",
			ArtistID:         "6194d02002d6ed297500ec7c04ad07d8",
			AlbumID:          "746632dea6742ff1234b62ba3505ec3c",
			AlbumArtistID:    "6194d02002d6ed297500ec7c04ad07d8",
			MbzTrackID:       "00fa0f1d-21e1-4611-b0a8-c6ec37cdea1a",
			MbzAlbumID:       "9b085411-6ab5-30e3-b2a4-8c6049ff8a37",
			MbzArtistID:      "2449300a-6ca7-45da-8102-22789d256475",
			MbzAlbumArtistID: "2449300a-6ca7-45da-8102-22789d256475",
			TrackNumber:      1,
		},
		{
			ID:               "1c7555c6d5ce4ddc35290c473c60cdea",
			Title:            "March of Mephisto (radio edit)",
			ArtistID:         "6194d02002d6ed297500ec7c04ad07d8",
			AlbumID:          "746632dea6742ff1234b62ba3505ec3c",
			AlbumArtistID:    "6194d02002d6ed297500ec7c04ad07d8",
			MbzTrackID:       "47f01f61-e38e-40e7-9fd7-c21f46f407c7",
			MbzAlbumID:       "9b085411-6ab5-30e3-b2a4-8c6049ff8a37",
			MbzArtistID:      "2449300a-6ca7-45da-8102-22789d256475",
			MbzAlbumArtistID: "2449300a-6ca7-45da-8102-22789d256475",
			TrackNumber:      16,
		},
	}

	jpTracks := []*model.MediaFile{
		{
			ID:               "90fede51aa7c28e53d7ff5a92f0b4976",
			Title:            "March of Mephisto",
			ArtistID:         "6194d02002d6ed297500ec7c04ad07d8",
			AlbumID:          "746632dea6742ff1234b62ba3505ec3c",
			AlbumArtistID:    "6194d02002d6ed297500ec7c04ad07d8",
			MbzTrackID:       "00fa0f1d-21e1-4611-b0a8-c6ec37cdea1a",
			MbzAlbumID:       "ec77f421-084d-4c0e-9966-257a549823d3",
			MbzArtistID:      "2449300a-6ca7-45da-8102-22789d256475",
			MbzAlbumArtistID: "2449300a-6ca7-45da-8102-22789d256475",
			TrackNumber:      1,
		},
		{
			ID:               "fc9fb4415c64d84edd0ce5198779fe60",
			Title:            "Soul Society (radio edit version)",
			ArtistID:         "6194d02002d6ed297500ec7c04ad07d8",
			AlbumID:          "746632dea6742ff1234b62ba3505ec3c",
			AlbumArtistID:    "6194d02002d6ed297500ec7c04ad07d8",
			MbzTrackID:       "a33f4144-a260-4be9-af21-f695a0f6fce4",
			MbzAlbumID:       "ec77f421-084d-4c0e-9966-257a549823d3",
			MbzArtistID:      "2449300a-6ca7-45da-8102-22789d256475",
			MbzAlbumArtistID: "2449300a-6ca7-45da-8102-22789d256475",
			TrackNumber:      16,
		},
	}

	trackRepo := ds.MediaFile(ctx)
	for _, mf := range brazilTracks {
		_ = trackRepo.Put(mf)
	}

	for _, mf := range jpTracks {
		_ = trackRepo.Put(mf)
	}

	return &ds
}

func TestArtistMbzIDMigrations(t *testing.T) {

}

func TestAlbumMbzIDMigrations(t *testing.T) {

}

func TestMediaFileMbzIDMigrations(t *testing.T) {

}

func TestFullExampleMbzIDMigration(t *testing.T) {
	var err error

	ctx := context.Background()
	ds := createKamelotExample()

	err = migrateArtists(ctx, ds)
	assert.NoError(t, err)
	err = migrateAlbums(ctx, ds)
	assert.NoError(t, err)
	err = migrateMediaFiles(ctx, ds)
	assert.NoError(t, err)

	_, err = ds.Artist(ctx).Get("6194d02002d6ed297500ec7c04ad07d8")
	assert.ErrorIs(t, err, model.ErrNotFound)

	artist, err := ds.Artist(ctx).Get("2449300a-6ca7-45da-8102-22789d256475")
	assert.NoError(t, err)

	assert.Equal(t, &model.Artist{
		ID:          "2449300a-6ca7-45da-8102-22789d256475",
		Name:        "Kamelot",
		MbzArtistID: "2449300a-6ca7-45da-8102-22789d256475",
	}, artist)

	_, err = ds.Album(ctx).Get("746632dea6742ff1234b62ba3505ec3c")
	assert.ErrorIs(t, err, model.ErrNotFound)

	brAlbum, err := ds.Album(ctx).Get("9b085411-6ab5-30e3-b2a4-8c6049ff8a37")
	assert.NoError(t, err)
	assert.Equal(t, &model.Album{
		ID:               "9b085411-6ab5-30e3-b2a4-8c6049ff8a37",
		Name:             "The Black Halo",
		ArtistID:         "2449300a-6ca7-45da-8102-22789d256475",
		MbzAlbumID:       "9b085411-6ab5-30e3-b2a4-8c6049ff8a37",
		MbzAlbumArtistID: "2449300a-6ca7-45da-8102-22789d256475",
		AlbumArtistID:    "2449300a-6ca7-45da-8102-22789d256475",
		AllArtistIDs:     "2449300a-6ca7-45da-8102-22789d256475",
	}, brAlbum)

	jpAlbum, err := ds.Album(ctx).Get("ec77f421-084d-4c0e-9966-257a549823d3")
	assert.NoError(t, err)
	assert.Equal(t, &model.Album{
		ID:               "ec77f421-084d-4c0e-9966-257a549823d3",
		Name:             "The Black Halo",
		ArtistID:         "2449300a-6ca7-45da-8102-22789d256475",
		MbzAlbumID:       "ec77f421-084d-4c0e-9966-257a549823d3",
		MbzAlbumArtistID: "2449300a-6ca7-45da-8102-22789d256475",
		AlbumArtistID:    "2449300a-6ca7-45da-8102-22789d256475",
		AllArtistIDs:     "2449300a-6ca7-45da-8102-22789d256475",
	}, jpAlbum)

	// Brazil "March of Mephisto"
	brMarch, err := ds.MediaFile(ctx).Get("9b085411-6ab5-30e3-b2a4-8c6049ff8a37-00fa0f1d-21e1-4611-b0a8-c6ec37cdea1a")
	assert.NoError(t, err)
	assert.Equal(t, &model.MediaFile{
		ID:               "9b085411-6ab5-30e3-b2a4-8c6049ff8a37-00fa0f1d-21e1-4611-b0a8-c6ec37cdea1a",
		Title:            "March of Mephisto",
		ArtistID:         "2449300a-6ca7-45da-8102-22789d256475",
		AlbumID:          "9b085411-6ab5-30e3-b2a4-8c6049ff8a37",
		AlbumArtistID:    "2449300a-6ca7-45da-8102-22789d256475",
		MbzTrackID:       "00fa0f1d-21e1-4611-b0a8-c6ec37cdea1a",
		MbzAlbumID:       "9b085411-6ab5-30e3-b2a4-8c6049ff8a37",
		MbzArtistID:      "2449300a-6ca7-45da-8102-22789d256475",
		MbzAlbumArtistID: "2449300a-6ca7-45da-8102-22789d256475",
		TrackNumber:      1,
	}, brMarch)

	_, err = ds.MediaFile(ctx).Get("1b28d36bd21837c24d5ad766a21a345d")
	assert.ErrorIs(t, err, model.ErrNotFound)

	// Brazil "March of Mephisto (radio edit)"
	brMarchRadio, err := ds.MediaFile(ctx).Get("9b085411-6ab5-30e3-b2a4-8c6049ff8a37-47f01f61-e38e-40e7-9fd7-c21f46f407c7")
	assert.NoError(t, err)
	assert.Equal(t, &model.MediaFile{
		ID:               "9b085411-6ab5-30e3-b2a4-8c6049ff8a37-47f01f61-e38e-40e7-9fd7-c21f46f407c7",
		Title:            "March of Mephisto (radio edit)",
		ArtistID:         "2449300a-6ca7-45da-8102-22789d256475",
		AlbumID:          "9b085411-6ab5-30e3-b2a4-8c6049ff8a37",
		AlbumArtistID:    "2449300a-6ca7-45da-8102-22789d256475",
		MbzTrackID:       "47f01f61-e38e-40e7-9fd7-c21f46f407c7",
		MbzAlbumID:       "9b085411-6ab5-30e3-b2a4-8c6049ff8a37",
		MbzArtistID:      "2449300a-6ca7-45da-8102-22789d256475",
		MbzAlbumArtistID: "2449300a-6ca7-45da-8102-22789d256475",
		TrackNumber:      16,
	}, brMarchRadio)

	// Japan "March of Mephisto"
	jpMarch, err := ds.MediaFile(ctx).Get("ec77f421-084d-4c0e-9966-257a549823d3-00fa0f1d-21e1-4611-b0a8-c6ec37cdea1a")
	assert.NoError(t, err)
	assert.Equal(t, &model.MediaFile{
		ID:               "ec77f421-084d-4c0e-9966-257a549823d3-00fa0f1d-21e1-4611-b0a8-c6ec37cdea1a",
		Title:            "March of Mephisto",
		ArtistID:         "2449300a-6ca7-45da-8102-22789d256475",
		AlbumID:          "ec77f421-084d-4c0e-9966-257a549823d3",
		AlbumArtistID:    "2449300a-6ca7-45da-8102-22789d256475",
		MbzTrackID:       "00fa0f1d-21e1-4611-b0a8-c6ec37cdea1a",
		MbzAlbumID:       "ec77f421-084d-4c0e-9966-257a549823d3",
		MbzArtistID:      "2449300a-6ca7-45da-8102-22789d256475",
		MbzAlbumArtistID: "2449300a-6ca7-45da-8102-22789d256475",
		TrackNumber:      1,
	}, jpMarch)

	_, err = ds.MediaFile(ctx).Get("90fede51aa7c28e53d7ff5a92f0b4976")
	assert.ErrorIs(t, err, model.ErrNotFound)

	// Japan "Soul Society (radio edit version)"
	jpSoul, err := ds.MediaFile(ctx).Get("ec77f421-084d-4c0e-9966-257a549823d3-a33f4144-a260-4be9-af21-f695a0f6fce4")
	assert.NoError(t, err)
	assert.Equal(t, &model.MediaFile{
		ID:               "ec77f421-084d-4c0e-9966-257a549823d3-a33f4144-a260-4be9-af21-f695a0f6fce4",
		Title:            "Soul Society (radio edit version)",
		ArtistID:         "2449300a-6ca7-45da-8102-22789d256475",
		AlbumID:          "ec77f421-084d-4c0e-9966-257a549823d3",
		AlbumArtistID:    "2449300a-6ca7-45da-8102-22789d256475",
		MbzTrackID:       "a33f4144-a260-4be9-af21-f695a0f6fce4",
		MbzAlbumID:       "ec77f421-084d-4c0e-9966-257a549823d3",
		MbzArtistID:      "2449300a-6ca7-45da-8102-22789d256475",
		MbzAlbumArtistID: "2449300a-6ca7-45da-8102-22789d256475",
		TrackNumber:      16,
	}, jpSoul)

}
