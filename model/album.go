package model

import "time"

type Album struct {
	Annotations `structs:"-"`

	ID                   string    `structs:"id" json:"id"            orm:"column(id)"`
	Name                 string    `structs:"name" json:"name"`
	CoverArtPath         string    `structs:"cover_art_path" json:"coverArtPath"`
	CoverArtId           string    `structs:"cover_art_id" json:"coverArtId"`
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
	CreatedAt            time.Time `structs:"created_at" json:"createdAt"`
	UpdatedAt            time.Time `structs:"updated_at" json:"updatedAt"`
}

type (
	Albums []Album
	DiscID struct {
		AlbumID    string `json:"albumId"`
		DiscNumber int    `json:"discNumber"`
	}
)

type AlbumRepository interface {
	CountAll(...QueryOptions) (int64, error)
	Exists(id string) (bool, error)
	Put(*Album) error
	Get(id string) (*Album, error)
	GetAll(...QueryOptions) (Albums, error)
	GetAllWithoutGenres(...QueryOptions) (Albums, error)
	Search(q string, offset int, size int) (Albums, error)
	Refresh(ids ...string) error
	DeleteMany(ids ...string) error
	AnnotatedRepository
}
