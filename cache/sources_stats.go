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
	GetChildren(uid string) []string
	GetHistogram() ClassByTypeStats
}

type SourcesStatsCacheImpl struct {
	mdb  *sql.DB
	tree *StatsTree
}

func NewSourcesStatsCacheImpl(mdb *sql.DB) SourcesStatsCache {
	ssc := new(SourcesStatsCacheImpl)
	ssc.mdb = mdb
	return ssc
}

func (ssc *SourcesStatsCacheImpl) Refresh() error {
	err := ssc.load()
	return errors.Wrap(err, "Load tags and sources stats.")
}

func (ssc *SourcesStatsCacheImpl) GetHistogram() ClassByTypeStats {
	return ssc.tree.flatten()
}

func (ssc *SourcesStatsCacheImpl) GetChildren(rootUID string) []string {
	root := ssc.tree.byUID[rootUID]
	return ssc.getAllChildren(root, []string{})
}

func (ssc *SourcesStatsCacheImpl) getAllChildren(root *StatsNode, result []string) []string {
	if root == nil {
		return result
	}
	result = append(result, root.uid)
	if root.children == nil {
		return result
	}
	for _, id := range root.children {
		ch := ssc.tree.byID[id]
		ssc.getAllChildren(ch, result)
	}
	return result
}

func (ssc *SourcesStatsCacheImpl) load() error {
	rows, err := queries.Raw(ssc.mdb, `
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
		var k string
		var id int64
		var typeID, parentID null.Int64
		var count int
		err = rows.Scan(&id, &parentID, &k, &typeID, &count)
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

		sources.insert(id, parentID.Int64, k[1:], ctName, count)
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "rows.Err()")
	}

	sources.accumulate()
	sources.flatten()
	ssc.tree = sources
	return nil
}
