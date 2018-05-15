package cache

import (
	"database/sql"
	"fmt"

	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/queries"
	"gopkg.in/volatiletech/null.v6"

	"github.com/Bnei-Baruch/archive-backend/mdb"
)

type Histogram map[string]int

func (h Histogram) Increment(k string, v int) {
	h[k] += v
}

func (h Histogram) Merge(other Histogram) {
	for k, v := range other {
		h.Increment(k, v)
	}
}

type StatsNode struct {
	id       int64
	parentID int64
	uid      string
	children []int64
	hist     Histogram
}

type StatsTree struct {
	byID map[int64]*StatsNode
}

func NewStatsTree() *StatsTree {
	st := new(StatsTree)
	st.byID = make(map[int64]*StatsNode)
	return st
}

func (st *StatsTree) accumulate() {
	// s starts as all leaf nodes
	s := make([]int64, 0)
	for k, v := range st.byID {
		if len(v.children) == 0 {
			s = append(s, k)
		}
	}

	// while we have some parents to collapse
	for len(s) > 0 {
		parents := make(map[int64]bool)

		for i := range s {
			node := st.byID[s[i]]
			if node.parentID != 0 {
				p := st.byID[node.parentID] // get parent
				parents[p.id] = true        // add to next gen of
				p.hist.Merge(node.hist)     // merge parent histogram with that of child
			}
		}

		// map -> slice (next gen of parents)
		s = make([]int64, len(parents))
		i := 0
		for k := range parents {
			s[i] = k
			i++
		}
	}
}

func (st *StatsTree) flatten() map[string]Histogram {
	byUID := make(map[string]Histogram, len(st.byID))
	for _, v := range st.byID {
		byUID[v.uid] = v.hist
	}
	return byUID
}

func (st *StatsTree) insert(id, parentID int64, uid string, ct string, cnt int) {
	node, ok := st.byID[id]
	if !ok {
		node = new(StatsNode)
		node.id = id
		node.parentID = parentID
		node.uid = uid
		node.hist = make(Histogram)
		st.byID[id] = node
	}
	if ct != "" {
		node.hist.Increment(ct, cnt)
	}
}

type ClassByTypeStats map[string]Histogram

func (s ClassByTypeStats) dump() {
	fmt.Printf("%d entries\n", len(s))
	for k, v := range s {
		fmt.Printf("%s\t\t%+v\n", k, v)
	}
}

type SearchStatsCache interface {
	Provider
	IsTagWithUnits(uid string, cts ...string) bool
	IsSourceWithUnits(uid string, cts ...string) bool
}

type SearchStatsCacheImpl struct {
	mdb     *sql.DB
	tags    ClassByTypeStats
	sources ClassByTypeStats
}

func NewSearchStatsCacheImpl(mdb *sql.DB) SearchStatsCache {
	ssc := new(SearchStatsCacheImpl)
	ssc.mdb = mdb
	return ssc
}

func (ssc *SearchStatsCacheImpl) IsTagWithUnits(uid string, cts ...string) bool {
	return ssc.isClassWithUnits("tags", uid, cts...)
}

func (ssc *SearchStatsCacheImpl) IsSourceWithUnits(uid string, cts ...string) bool {
	return ssc.isClassWithUnits("sources", uid, cts...)
}

func (ssc *SearchStatsCacheImpl) isClassWithUnits(class, uid string, cts ...string) bool {
	var stats ClassByTypeStats
	switch class {
	case "tags":
		stats = ssc.tags
	case "sources":
		stats = ssc.sources
	default:
		return false
	}

	if h, ok := stats[uid]; ok {
		for i := range cts {
			if c, ok := h[cts[i]]; ok && c > 0 {
				return true
			}
		}
	}

	return false
}

func (ssc *SearchStatsCacheImpl) String() string {
	return "SearchStatsCacheImpl"
}

func (ssc *SearchStatsCacheImpl) Refresh() error {
	var err error
	ssc.tags, ssc.sources, err = ssc.load()
	if err != nil {
		return errors.Wrap(err, "Load stats")
	}

	return nil
}

func (ssc *SearchStatsCacheImpl) load() (ClassByTypeStats, ClassByTypeStats, error) {
	rows, err := queries.Raw(ssc.mdb, `select
  t.id,
  t.parent_id,
  concat('t', t.uid),
  cu.type_id,
  count(cu.id)
from tags t
  left join content_units_tags cut on t.id = cut.tag_id
  left join content_units cu on cut.content_unit_id = cu.id
group by t.id, cu.type_id
union
select
  s.id,
  s.parent_id,
  concat('s', s.uid),
  cu.type_id,
  count(cu.id)
from sources s
  left join content_units_sources cus on s.id = cus.source_id
  left join content_units cu on cus.content_unit_id = cu.id
group by s.id, cu.type_id;`).Query()
	if err != nil {
		return nil, nil, errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()

	// TODO: take authors into consideration

	tags := NewStatsTree()
	sources := NewStatsTree()
	var tmp *StatsTree
	for rows.Next() {
		var k string
		var id int64
		var typeID, parentID null.Int64
		var count int
		err = rows.Scan(&id, &parentID, &k, &typeID, &count)
		if err != nil {
			return nil, nil, errors.Wrap(err, "rows.Scan")
		}

		if k[0] == 't' {
			tmp = tags
		} else if k[0] == 's' {
			tmp = sources
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

		tmp.insert(id, parentID.Int64, k[1:], ctName, count)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, errors.Wrap(err, "rows.Err()")
	}

	tags.accumulate()
	sources.accumulate()

	return tags.flatten(), sources.flatten(), nil
}
