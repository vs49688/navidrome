package scanner

import (
	"context"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

func newCachedGenreRepository(ctx context.Context, repo model.GenreRepository) model.GenreRepository {
	r := &cachedGenreRepo{
		GenreRepository: repo,
		ctx:             ctx,
	}

	log.Debug("XXX: newCachedGenreResository() BEGIN")

	log.Debug("XXX: newCachedGenreResository()/GetAll() BEGIN")
	genres, err := repo.GetAll()
	log.Debug("XXX: newCachedGenreResository()/GetAll() END", "count", len(genres), err)
	if err != nil {
		log.Error(ctx, "Could not load genres from DB", err)
		log.Debug("XXX: newCachedGenreResository() END", err)
		return repo
	}

	r.cache = ttlcache.NewCache()
	for _, g := range genres {
		log.Debug("XXX: newCachedGenreResository() CACHE", "name", strings.ToLower(g.Name), "id", g.ID)
		_ = r.cache.Set(strings.ToLower(g.Name), g.ID)
	}

	log.Debug("XXX: newCachedGenreResository() CACHED", "count", len(genres))

	log.Debug("XXX: newCachedGenreResository() END/SUCCESS")
	return r
}

type cachedGenreRepo struct {
	model.GenreRepository
	cache *ttlcache.Cache
	ctx   context.Context
}

func (r *cachedGenreRepo) Put(g *model.Genre) error {
	log.Debug("XXX: cachedGenreRepo/Put() BEGIN", "id", g.ID, "name", g.Name)
	id, err := r.cache.GetByLoader(strings.ToLower(g.Name), func(key string) (interface{}, time.Duration, error) {
		log.Debug("XXX: cachedGenreRepo/Put() CACHE MISS", "id", g.ID, "name", g.Name)
		err := r.GenreRepository.Put(g)
		return g.ID, 24 * time.Hour, err
	})
	g.ID = id.(string)
	log.Debug("XXX: cachedGenreRepo/Put() END", "id", g.ID, "name", g.Name)
	return err
}
