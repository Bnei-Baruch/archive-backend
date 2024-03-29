package cache

import (
	"database/sql"

	"github.com/pkg/errors"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/queries"

	"github.com/Bnei-Baruch/archive-backend/mdb"
)

type SourcesStatsCache interface {
	Provider
	GetTree() *StatsTree
}

type SourcesStatsCacheImpl struct {
	mdb  *sql.DB
	tree *StatsTree
}

func (s *SourcesStatsCacheImpl) String() string {
	return "SourcesStatsCacheImpl"
}

func NewSourcesStatsCacheImpl(mdb *sql.DB) SourcesStatsCache {
	stats := new(SourcesStatsCacheImpl)
	stats.mdb = mdb
	stats.tree = NewStatsTree()
	return stats
}

func (s *SourcesStatsCacheImpl) GetTree() *StatsTree {
	return s.tree
}

func (s *SourcesStatsCacheImpl) Refresh() error {
	s.tree.resetTemp()
	err := s.load()
	return errors.Wrap(err, "Load sources stats.")
}

func (s *SourcesStatsCacheImpl) load() error {
	rows, err := queries.Raw(`
	SELECT
		s.id, s.parent_id, s.uid, cu.type_id, COUNT(cu.id)
	FROM sources s
	  LEFT JOIN content_units_sources cus ON s.id = cus.source_id
	  LEFT JOIN (
			SELECT * FROM content_units WHERE content_units.secure = 0 AND content_units.published IS TRUE
		) AS cu ON cus.content_unit_id = cu.id
	GROUP BY s.id, cu.type_id;
	`).Query(s.mdb)
	if err != nil {
		return errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()

	for rows.Next() {
		var uid string
		var id int64
		var typeID, parentID null.Int64
		var count int
		err = rows.Scan(&id, &parentID, &uid, &typeID, &count)
		if err != nil {
			return errors.Wrap(err, "rows.Scan")
		}

		var ctName = ""
		if typeID.Valid {
			ct, ok := mdb.CONTENT_TYPE_REGISTRY.ByID[typeID.Int64]
			if ok {
				ctName = ct.Name
			} else {
				continue
			}
		}

		s.tree.insert(id, parentID.Int64, uid, ctName, count)
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "rows.Err()")
	}

	s.tree.accumulate()
	s.tree.flatten()
	return nil
}
