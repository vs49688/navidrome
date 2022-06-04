package tests

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/navidrome/navidrome/model"
)

func CreateMockArtistRepo() *MockArtistRepo {
	return &MockArtistRepo{
		data:        make(map[string]*model.Artist),
		annotations: make(map[string]*model.Annotations),
	}
}

type MockArtistRepo struct {
	model.ArtistRepository
	data        map[string]*model.Artist
	annotations map[string]*model.Annotations
	err         bool
}

func (m *MockArtistRepo) SetError(err bool) {
	m.err = err
}

func (m *MockArtistRepo) SetData(artists model.Artists) {
	m.data = make(map[string]*model.Artist)
	for i, a := range artists {
		m.data[a.ID] = &artists[i]
	}
}

func (m *MockArtistRepo) Exists(id string) (bool, error) {
	if m.err {
		return false, errors.New("Error!")
	}
	_, found := m.data[id]
	return found, nil
}

func (m *MockArtistRepo) Get(id string) (*model.Artist, error) {
	if m.err {
		return nil, errors.New("Error!")
	}
	if d, ok := m.data[id]; ok {
		return d, nil
	}
	return nil, model.ErrNotFound
}

func (m *MockArtistRepo) GetAll(options ...model.QueryOptions) (model.Artists, error) {
	// TODO: handle options

	if m.err {
		return nil, errors.New("Error!")
	}

	res := make([]model.Artist, 0, len(m.data))
	for _, a := range m.data {
		res = append(res, *a)
	}

	return res, nil
}

func (m *MockArtistRepo) Put(ar *model.Artist) error {
	if m.err {
		return errors.New("error")
	}

	artistCopy := &model.Artist{}
	*artistCopy = *ar
	if artistCopy.ID == "" {
		artistCopy.ID = uuid.NewString()
	}
	m.data[ar.ID] = artistCopy
	return nil
}

func (m *MockArtistRepo) IncPlayCount(id string, timestamp time.Time) error {
	if m.err {
		return errors.New("error")
	}
	if d, ok := m.data[id]; ok {
		d.PlayCount++
		d.PlayDate = timestamp
		return nil
	}
	return model.ErrNotFound
}

func (m *MockArtistRepo) DeleteMany(ids ...string) error {
	for _, id := range ids {
		delete(m.data, id)
	}
	return nil
}

func (m *MockArtistRepo) MoveAnnotation(fromID string, toID string) error {
	from, found := m.annotations[fromID]
	if !found {
		return nil
	}

	m.annotations[toID] = from
	return nil
}

var _ model.ArtistRepository = (*MockArtistRepo)(nil)
