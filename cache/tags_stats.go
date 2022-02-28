package cache

import (
	"database/sql"

	"github.com/Bnei-Baruch/sqlboiler/queries"
	"github.com/pkg/errors"
	"gopkg.in/volatiletech/null.v6"

	"github.com/Bnei-Baruch/archive-backend/mdb"
)

type TagsStatsCache interface {
	Provider
	GetTree() *StatsTree
}

type TagsStatsCacheImpl struct {
	mdb  *sql.DB
	tree *StatsTree
}

func (s *TagsStatsCacheImpl) String() string {
	return "TagsStatsCacheImpl"
}

func NewTagsStatsCacheImpl(mdb *sql.DB) TagsStatsCache {
	stats := new(TagsStatsCacheImpl)
	stats.mdb = mdb
	stats.tree = NewStatsTree()
	return stats
}

func (s *TagsStatsCacheImpl) GetTree() *StatsTree {
	return s.tree
}

func (s *TagsStatsCacheImpl) Refresh() error {
	s.tree.resetTemp()
	err := s.load()
	return errors.Wrap(err, "Load tags stats.")
}

func (s *TagsStatsCacheImpl) load() error {
	rows, err := queries.Raw(s.mdb, `
		SELECT
			t.id, t.parent_id, t.uid, cu.type_id, COUNT(cu.id) 
		FROM tags t
			LEFT JOIN content_units_tags cut ON t.id = cut.tag_id
			LEFT JOIN (
					SELECT * FROM content_units WHERE content_units.secure = 0 AND content_units.published IS TRUE
				) AS cu ON cut.content_unit_id = cu.id
		GROUP BY t.id, cu.type_id
	`).Query()
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
