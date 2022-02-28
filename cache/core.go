package cache

import (
	"database/sql"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

type Refreshable interface {
	Refresh() error
}

type Provider interface {
	Refreshable
	String() string
}

type CacheManager interface {
	SearchStats() SearchStatsCache
	SourcesStats() SourcesStatsCache
	AuthorsStats() AuthorsStatsCache
	TagsStats() TagsStatsCache
	Close()
}

type CacheManagerImpl struct {
	search           SearchStatsCache
	sources          SourcesStatsCache
	authors          AuthorsStatsCache
	tags             TagsStatsCache
	ticker           *time.Ticker
	ticks            int64
	refreshIntervals map[string]int64
	providers        []Provider
}

func NewCacheManagerImpl(mdb *sql.DB, refreshIntervals map[string]time.Duration) CacheManager {
	cm := new(CacheManagerImpl)
	cm.sources = NewSourcesStatsCacheImpl(mdb)
	cm.tags = NewTagsStatsCacheImpl(mdb)
	cm.authors = NewAuthorsStatsCacheImpl(mdb)
	cm.providers = []Provider{cm.sources, cm.tags, cm.authors}

	cm.refresh()
	cm.search = NewSearchStatsCacheImpl(mdb, cm.sources.GetTree().flatten(), cm.tags.GetTree().flatten())
	cm.providers = append(cm.providers, cm.search)
	if err := cm.search.Refresh(); err != nil {
		log.Errorf("Refresh %s: %s", cm.search, err.Error())
		utils.LogError(err)
	}

	// Convert time.Duration to int64
	// So we would have refresh intervals in integer multiple of a second
	cm.refreshIntervals = make(map[string]int64, len(refreshIntervals))
	for k, v := range refreshIntervals {
		cm.refreshIntervals[k] = int64(v.Truncate(time.Second).Seconds())
	}

	cm.ticker = time.NewTicker(time.Second)
	go func() {
		for range cm.ticker.C {
			cm.ticks++
			if cm.ticks%cm.refreshIntervals["SearchStats"] == 0 {
				cm.refresh()
			}
		}
	}()

	return cm
}

func (cm *CacheManagerImpl) Close() {
	cm.ticker.Stop()
}

func (cm *CacheManagerImpl) SourcesStats() SourcesStatsCache {
	return cm.sources
}
func (cm *CacheManagerImpl) AuthorsStats() AuthorsStatsCache {
	return cm.authors
}

func (cm *CacheManagerImpl) TagsStats() TagsStatsCache {
	return cm.tags
}

func (cm *CacheManagerImpl) SearchStats() SearchStatsCache {
	return cm.search
}

func (cm *CacheManagerImpl) refresh() {
	for _, p := range cm.providers {
		log.Infof("Refreshing %s", p)
		if err := p.Refresh(); err != nil {
			log.Errorf("Refresh %s: %s", p, err.Error())
			utils.LogError(err)
		}
	}
}
