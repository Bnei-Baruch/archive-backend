package cache

import (
	"database/sql"

	"github.com/Bnei-Baruch/sqlboiler/queries"
	"github.com/pkg/errors"
	"gopkg.in/volatiletech/null.v6"

	"github.com/Bnei-Baruch/archive-backend/mdb"
)

type SourcesStatsCache interface {
	Refresh() error
	GetChildren(rootUIDs []string) ([]string, []int64)
	GetHistogram() ClassByTypeStats
}

type SourcesStatsCacheImpl struct {
	mdb  *sql.DB
	tree *StatsTree
}

func NewSourcesStatsCacheImpl(mdb *sql.DB) SourcesStatsCache {
	stats := new(SourcesStatsCacheImpl)
	stats.mdb = mdb
	return stats
}

func (s *SourcesStatsCacheImpl) Refresh() error {
	err := s.load()
	return errors.Wrap(err, "Load tags and sources stats.")
}

func (s *SourcesStatsCacheImpl) GetHistogram() ClassByTypeStats {
	return s.tree.flatten()
}

func (s *SourcesStatsCacheImpl) GetChildren(rootUIDs []string) ([]string, []int64) {
	chs := make([]*StatsNode, 0)
	for _, rootUID := range rootUIDs {
		root := s.tree.byUID[rootUID]
		chs = append(chs, s.getAllChildren(root)...)
	}
	uids := make([]string, len(chs))
	ids := make([]int64, len(chs))
	for i, ch := range chs {
		uids[i] = ch.uid
		ids[i] = ch.id
	}
	return uids, ids
}

func (s *SourcesStatsCacheImpl) getAllChildren(root *StatsNode) []*StatsNode {
	if root == nil {
		return make([]*StatsNode, 0)
	}
	result := []*StatsNode{root}
	if root.children == nil {
		return result
	}
	for _, id := range root.children {
		ch := s.tree.byID[id]
		result = append(result, s.getAllChildren(ch)...)
	}
	return result
}

func (s *SourcesStatsCacheImpl) load() error {
	rows, err := queries.Raw(s.mdb, `
	SELECT
		s.id, s.parent_id, s.uid, cu.type_id, COUNT(cu.id)
	FROM sources s
	  LEFT JOIN content_units_sources cus ON s.id = cus.source_id
	  LEFT JOIN (
			SELECT * FROM content_units WHERE content_units.secure = 0 AND content_units.published IS TRUE
		) AS cu ON cus.content_unit_id = cu.id
	GROUP BY s.id, cu.type_id;
	`).Query()
	if err != nil {
		return errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()

	sources := NewStatsTree()
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

		sources.insert(id, parentID.Int64, uid, ctName, count)
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "rows.Err()")
	}

	sources.accumulate()
	sources.flatten()
	s.tree = sources
	return nil
}
