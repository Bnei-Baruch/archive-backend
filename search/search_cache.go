package search

import(
	"time"

	log "github.com/Sirupsen/logrus"
  "github.com/jellydator/ttlcache/v2"
)

type SearchCache struct {
  Cache ttlcache.SimpleCache
}

func (c *SearchCache) Get(query Query, sortBy string, from int, size int) (*QueryResult, error) {
  key := query.ToFullSimpleString(sortBy, from, size)
  res, err := c.Cache.Get(key);
  if err == ttlcache.ErrNotFound {
    return nil, nil
  }
  if err != nil {
    return nil, err
  }
  log.Infof("Using cached result for %+v", key);
  return res.(*QueryResult), nil
}

func (c *SearchCache) Set(query Query, sortBy string, from int, size int, res *QueryResult) {
  key := query.ToFullSimpleString(sortBy, from, size)
  c.Cache.Set(key, res)
}

func MakeSearchCache() *SearchCache {
  cache := ttlcache.NewCache()
  cache.SetTTL(time.Duration(1 * time.Minute))
  cache.SetCacheSizeLimit(1000)
	return &SearchCache{Cache: cache}
}
