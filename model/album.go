package model

import (
	"time"

	"github.com/navidrome/navidrome/utils/slice"
	"golang.org/x/exp/slices"
)

type Album struct {
	Annotations `structs:"-"`

	ID                   string    `structs:"id" json:"id"            orm:"column(id)"`
	Name                 string    `structs:"name" json:"name"`
	EmbedArtPath         string    `structs:"embed_art_path" json:"embedArtPath"`
	ArtistID             string    `structs:"artist_id" json:"artistId"      orm:"column(artist_id)"`
	Artist               string    `structs:"artist" json:"artist"`
	AlbumArtistID        string    `structs:"album_artist_id" json:"albumArtistId" orm:"column(album_artist_id)"`
	AlbumArtist          string    `structs:"album_artist" json:"albumArtist"`
	AllArtistIDs         string    `structs:"all_artist_ids" json:"allArtistIds"  orm:"column(all_artist_ids)"`
	MaxYear              int       `structs:"max_year" json:"maxYear"`
	MinYear              int       `structs:"min_year" json:"minYear"`
	Compilation          bool      `structs:"compilation" json:"compilation"`
	Comment              string    `structs:"comment" json:"comment,omitempty"`
	SongCount            int       `structs:"song_count" json:"songCount"`
	Duration             float32   `structs:"duration" json:"duration"`
	Size                 int64     `structs:"size" json:"size"`
	Genre                string    `structs:"genre" json:"genre"`
	Genres               Genres    `structs:"-" json:"genres"`
	FullText             string    `structs:"full_text" json:"fullText"`
	SortAlbumName        string    `structs:"sort_album_name" json:"sortAlbumName,omitempty"`
	SortArtistName       string    `structs:"sort_artist_name" json:"sortArtistName,omitempty"`
	SortAlbumArtistName  string    `structs:"sort_album_artist_name" json:"sortAlbumArtistName,omitempty"`
	OrderAlbumName       string    `structs:"order_album_name" json:"orderAlbumName"`
	OrderAlbumArtistName string    `structs:"order_album_artist_name" json:"orderAlbumArtistName"`
	CatalogNum           string    `structs:"catalog_num" json:"catalogNum,omitempty"`
	MbzAlbumID           string    `structs:"mbz_album_id" json:"mbzAlbumId,omitempty"         orm:"column(mbz_album_id)"`
	MbzAlbumArtistID     string    `structs:"mbz_album_artist_id" json:"mbzAlbumArtistId,omitempty"   orm:"column(mbz_album_artist_id)"`
	MbzAlbumType         string    `structs:"mbz_album_type" json:"mbzAlbumType,omitempty"`
	MbzAlbumComment      string    `structs:"mbz_album_comment" json:"mbzAlbumComment,omitempty"`
	ImageFiles           string    `structs:"image_files" json:"imageFiles,omitempty"`
	Paths                string    `structs:"paths" json:"paths,omitempty"`
	CreatedAt            time.Time `structs:"created_at" json:"createdAt"`
	UpdatedAt            time.Time `structs:"updated_at" json:"updatedAt"`
}

func (a Album) CoverArtID() ArtworkID {
	return artworkIDFromAlbum(a)
}

type DiscID struct {
	AlbumID    string `json:"albumId"`
	DiscNumber int    `json:"discNumber"`
}

type Albums []Album

// ToAlbumArtist creates an Artist object based on the attributes of this Albums collection.
// It assumes all albums have the same AlbumArtist, or else results are unpredictable.
func (als Albums) ToAlbumArtist() Artist {
	a := Artist{AlbumCount: len(als)}
	var mbzArtistIds []string
	for _, al := range als {
		a.ID = al.AlbumArtistID
		a.Name = al.AlbumArtist
		a.SortArtistName = al.SortAlbumArtistName
		a.OrderArtistName = al.OrderAlbumArtistName

		a.SongCount += al.SongCount
		a.Size += al.Size
		a.Genres = append(a.Genres, al.Genres...)
		mbzArtistIds = append(mbzArtistIds, al.MbzAlbumArtistID)
	}
	slices.SortFunc(a.Genres, func(a, b Genre) bool { return a.ID < b.ID })
	a.Genres = slices.Compact(a.Genres)
	a.MbzArtistID = slice.MostFrequent(mbzArtistIds)

	return a
}

type AlbumRepository interface {
	CountAll(...QueryOptions) (int64, error)
	Exists(id string) (bool, error)
	Put(*Album) error
	Get(id string) (*Album, error)
	GetAll(...QueryOptions) (Albums, error)
	GetAllWithoutGenres(...QueryOptions) (Albums, error)
	Search(q string, offset int, size int) (Albums, error)
	DeleteMany(ids ...string) error
	AnnotatedRepository
}
