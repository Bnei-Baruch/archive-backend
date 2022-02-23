package cache

import (
	"database/sql"

	"github.com/Bnei-Baruch/sqlboiler/queries"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/Bnei-Baruch/archive-backend/mdb"
)

type AuthorsStatsCache interface {
	Provider
	GetSources(codes []string) []string
}

type AuthorsStatsCacheImpl struct {
	mdb    *sql.DB
	byCode map[string][]string
}

func NewAuthorsStatsCacheImpl(mdbDB *sql.DB) AuthorsStatsCache {
	stats := new(AuthorsStatsCacheImpl)
	stats.mdb = mdbDB
	stats.byCode = make(map[string][]string)
	for c, _ := range mdb.AUTHOR_REGISTRY.ByCode {
		stats.byCode[c] = make([]string, 0)
	}
	return stats
}

func (s *AuthorsStatsCacheImpl) String() string {
	return "AuthorsStatsCacheImpl"
}

func (s *AuthorsStatsCacheImpl) Refresh() error {
	err := s.load()
	return errors.Wrap(err, "Load authors stats.")
}

func (s *AuthorsStatsCacheImpl) load() error {
	rows, err := queries.Raw(s.mdb, `
		SELECT a.code, array_agg(DISTINCT s.uid) FROM authors_sources "as"
			INNER JOIN authors a ON "as".author_id = a.id
			INNER JOIN sources s ON "as".source_id = s.id
		GROUP BY a.code
	`).Query()
	if err != nil {
		return errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()

	for rows.Next() {
		var code string
		var uids pq.StringArray
		err := rows.Scan(&code, &uids)
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
