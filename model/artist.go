package model

import "time"

type Artist struct {
	Annotations `structs:"-"`

	ID                    string    `structs:"id" json:"id"               orm:"column(id)"`
	Name                  string    `structs:"name" json:"name"`
	AlbumCount            int       `structs:"album_count" json:"albumCount"`
	SongCount             int       `structs:"song_count" json:"songCount"`
	Genres                Genres    `structs:"-" json:"genres"`
	FullText              string    `structs:"full_text" json:"fullText"`
	SortArtistName        string    `structs:"sort_artist_name" json:"sortArtistName,omitempty"`
	OrderArtistName       string    `structs:"order_artist_name" json:"orderArtistName"`
	Size                  int64     `structs:"size" json:"size"`
	MbzArtistID           string    `structs:"mbz_artist_id" json:"mbzArtistId,omitempty"      orm:"column(mbz_artist_id)"`
	Biography             string    `structs:"biography" json:"biography,omitempty"`
	SmallImageUrl         string    `structs:"small_image_url" json:"smallImageUrl,omitempty"`
	MediumImageUrl        string    `structs:"medium_image_url" json:"mediumImageUrl,omitempty"`
	LargeImageUrl         string    `structs:"large_image_url" json:"largeImageUrl,omitempty"`
	ExternalUrl           string    `structs:"external_url" json:"externalUrl,omitempty"      orm:"column(external_url)"`
	SimilarArtists        Artists   `structs:"-"  json:"-"   orm:"-"`
	ExternalInfoUpdatedAt time.Time `structs:"external_info_updated_at" json:"externalInfoUpdatedAt"`
}

func (a Artist) ArtistImageUrl() string {
	if a.MediumImageUrl != "" {
		return a.MediumImageUrl
	}
	if a.LargeImageUrl != "" {
		return a.LargeImageUrl
	}
	return a.SmallImageUrl
}

type Artists []Artist

type ArtistIndex struct {
	ID      string
	Artists Artists
}
type ArtistIndexes []ArtistIndex

type ArtistRepository interface {
	CountAll(options ...QueryOptions) (int64, error)
	Exists(id string) (bool, error)
	Put(m *Artist) error
	Get(id string) (*Artist, error)
	GetAll(options ...QueryOptions) (Artists, error)
	Search(q string, offset int, size int) (Artists, error)
	Refresh(ids ...string) error
	GetIndex() (ArtistIndexes, error)
	DeleteMany(ids ...string) error
	AnnotatedRepository
}
