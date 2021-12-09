package cache

import (
	"database/sql"
	"time"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

type CacheManager interface {
	SearchStats() SearchStatsCache
	Close()
}

type CacheManagerImpl struct {
	search           SearchStatsCache
	sources          SourcesStatsCache
	tags             TagsStatsCache
	ticker           *time.Ticker
	ticks            int64
	refreshIntervals map[string]int64
}

func NewCacheManagerImpl(mdb *sql.DB, refreshIntervals map[string]time.Duration) CacheManager {
	cm := new(CacheManagerImpl)
	// Convert time.Duration to int64
	// So we would have refresh intervals in integer multiple of a second
	cm.refreshIntervals = make(map[string]int64, len(refreshIntervals))
	for k, v := range refreshIntervals {
		cm.refreshIntervals[k] = int64(v.Truncate(time.Second).Seconds())
	}

	cm.sources = NewSourcesStatsCacheImpl(mdb)
	cm.tags = NewTagsStatsCacheImpl(mdb)
	cm.search = NewSearchStatsCacheImpl(mdb)
	cm.refresh()

	cm.ticker = time.NewTicker(time.Second)
	go func() {
		for range cm.ticker.C {
			cm.ticks++
			cm.refresh()
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

func (cm *CacheManagerImpl) TagsStats() TagsStatsCache {
	return cm.tags
}

func (cm *CacheManagerImpl) SearchStats() SearchStatsCache {
	return cm.search
}

func (cm *CacheManagerImpl) refresh() {
	if cm.ticks%cm.refreshIntervals["TagAndSourcesStats"] == 0 {
		if err := cm.sources.Refresh(); err != nil {
			utils.LogError(err)
		}

		if err := cm.tags.Refresh(); err != nil {
			utils.LogError(err)
		}
	}

	if cm.ticks%cm.refreshIntervals["SearchStats"] == 0 {
		if err := cm.search.Refresh(cm.sources.GetHistogram(), cm.tags.GetHistogram()); err != nil {
			utils.LogError(err)
		}
	}
}
