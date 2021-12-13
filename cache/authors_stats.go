package cache

import (
	"database/sql"
	"time"

	"github.com/Bnei-Baruch/sqlboiler/queries"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/mdb"
)

type AuthorsStatsCache interface {
	Provider
	GetSources(codes []string) []string
}

type AuthorsStatsCacheImpl struct {
	mdb      *sql.DB
	byCode   map[string][]string
	interval int64
}

func NewAuthorsStatsCacheImpl(mdbDB *sql.DB) AuthorsStatsCache {
	stats := new(AuthorsStatsCacheImpl)
	stats.mdb = mdbDB
	stats.byCode = make(map[string][]string)
	for c, _ := range mdb.AUTHOR_REGISTRY.ByCode {
		stats.byCode[c] = make([]string, 0)
	}
	// Convert time.Duration to int64
	// So we would have refresh intervals in integer multiple of a second
	viper.SetDefault("cache.refresh-sources-and-tags", 24*time.Hour)
	stats.interval = int64(viper.GetDuration("cache.refresh-search-stats").Truncate(time.Second).Seconds())
	return stats
}

func (s *AuthorsStatsCacheImpl) Interval() int64 {
	return s.interval
}

func (s *AuthorsStatsCacheImpl) String() string {
	return "AuthorsStatsCacheImpl"
}

func (s *AuthorsStatsCacheImpl) Refresh() error {
	err := s.load()
	return errors.Wrap(err, "Load tags and sources stats.")
}

func (s *AuthorsStatsCacheImpl) load() error {
	rows, err := queries.Raw(s.mdb, `
		SELECT a.code, array_agg(DISTINCT s.uid) FROM authors_sources "as"
			INNER JOIN sources s ON "as".source_id = s.id
			INNER JOIN authors a ON "as".author_id = a.id
		GROUP BY "as".author_id
	`).Query()
	if err != nil {
		return errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()

	for rows.Next() {
		var code string
		var uids pq.StringArray
		err := rows.Scan(code, &uids)
		if err != nil {
			return errors.Wrap(err, "rows.Scan")
		}
		s.byCode[code] = uids
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "rows.Err()")
	}

	return nil
}

func (s *AuthorsStatsCacheImpl) GetSources(codes []string) []string {
	ret := make([]string, 0)
	for _, c := range codes {
		ret = append(ret, s.byCode[c]...)
	}
	return ret
}
