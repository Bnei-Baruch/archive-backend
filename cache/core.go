package cache

import (
	"database/sql"
	"time"

	log "github.com/Sirupsen/logrus"
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
	Close()
}

type CacheManagerImpl struct {
	ssc              SearchStatsCache
	ticker           *time.Ticker
	ticks            int64
	refreshIntervals map[string]int64
}

func NewCacheManagerImpl(mdb *sql.DB, refreshIntervals map[string]time.Duration) CacheManager {
	cm := new(CacheManagerImpl)
	cm.ssc = NewSearchStatsCacheImpl(mdb)
	cm.refresh(cm.ssc)

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
				cm.refresh(cm.ssc)
			}
		}
	}()

	return cm
}

func (cm *CacheManagerImpl) Close() {
	cm.ticker.Stop()
}

func (cm *CacheManagerImpl) SearchStats() SearchStatsCache {
	return cm.ssc
}

func (cm *CacheManagerImpl) refresh(p Provider) {
	log.Infof("Refreshing %s", p)
	if err := p.Refresh(); err != nil {
		log.Errorf("Refresh %s: %s", p, err.Error())
	}
}
