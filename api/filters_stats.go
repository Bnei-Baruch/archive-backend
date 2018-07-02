package api

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/queries"
	"gopkg.in/volatiletech/null.v6"

	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
)

type ClassificationStats map[string]int

func (s ClassificationStats) dump() {
	fmt.Printf("%d entries\n", len(s))
	for k, v := range s {
		fmt.Printf("%s\t\t%+v\n", k, v)
	}
}

type IDSet map[int64]bool

func (h IDSet) Increment(k int64) {
	h[k] = true
}

func (h IDSet) Merge(other IDSet) {
	for k := range other {
		h.Increment(k)
	}
}

type StatsNode struct {
	id       int64
	parentID int64
	uid      string
	children []int64
	cuIDs    IDSet
}

type StatsTree struct {
	byID map[int64]*StatsNode
}

func NewStatsTree() *StatsTree {
	st := new(StatsTree)
	st.byID = make(map[int64]*StatsNode)
	return st
}

// accumulate merge Histograms bottom up so that
// parent nodes's Histogram will be the overall sum of its children.
// We do that in code because we don't really know how to do that with SQL.
func (st *StatsTree) accumulate() {
	// compute children since next step rely on it for correction
	for k, v := range st.byID {
		if v.parentID != 0 {
			parent := st.byID[v.parentID]
			parent.children = append(parent.children, k)
		}
	}

	// put all leaf nodes in s
	s := make([]int64, 0)
	for k, v := range st.byID {
		if len(v.children) == 0 {
			s = append(s, k)
		}
	}

	// while we have some nodes to merge
	for len(s) > 0 {
		// loop through this generation of nodes
		// merge parents ID sets and collect next generation
		parents := make(map[int64]bool)
		for i := range s {
			node := st.byID[s[i]]
			if node.parentID != 0 {
				p := st.byID[node.parentID] // get parent
				parents[p.id] = true        // add to next gen of
				p.cuIDs.Merge(node.cuIDs)   // merge parent ID set with that of child
			}
		}

		// convert next generation of nodes map to slice (parents of current generation)
		s = make([]int64, len(parents))
		i := 0
		for k := range parents {
			s[i] = k
			i++
		}
	}

	// add artificial root node
	root := new(StatsNode)
	root.uid = "root"
	root.cuIDs = make(IDSet)
	st.byID[-1] = root
	for _, v := range st.byID {
		if v.parentID == 0 {
			root.cuIDs.Merge(v.cuIDs)
		}
	}
}

// flatten return a flat uid => Histogram lookup table.
// It's usually the only interesting result to use
// as the tree structure is not really needed once accumulated.
func (st *StatsTree) flatten() ClassificationStats {
	byUID := make(ClassificationStats, len(st.byID))
	for _, v := range st.byID {
		if count := len(v.cuIDs); count > 0 {
			byUID[v.uid] = count
		}
	}
	return byUID
}

func (st *StatsTree) insert(id, parentID int64, uid string, ids []int64) {
	node, ok := st.byID[id]
	if !ok {
		node = new(StatsNode)
		node.id = id
		node.parentID = parentID
		node.uid = uid
		node.cuIDs = make(IDSet)
		st.byID[id] = node
	}
	for i := range ids {
		node.cuIDs.Increment(ids[i])
	}
}

func GetFiltersStats(db *sql.DB, cuScope string, cuScopeArgs []interface{}) (ClassificationStats, ClassificationStats, error) {
	qq := fmt.Sprintf(`with fcu as (%s)
select
  s.id,
  s.parent_id,
  concat('s', s.uid),
  array_agg(distinct cus.content_unit_id)
from sources s
  inner join content_units_sources cus on s.id = cus.source_id
  inner join fcu on cus.content_unit_id = fcu.id
group by s.id
union
select
  s.id,
  s.parent_id,
  concat('s', s.uid),
  '{}'
from sources s
union
select
  t.id,
  t.parent_id,
  concat('t', t.uid),
  array_agg(distinct cut.content_unit_id)
from tags t
  inner join content_units_tags cut on t.id = cut.tag_id
  inner join fcu on cut.content_unit_id = fcu.id
group by t.id
union
select
  t.id,
  t.parent_id,
  concat('t', t.uid),
  '{}'
from tags t`, cuScope[:len(cuScope)-1])

	rows, err := queries.Raw(db, qq, cuScopeArgs...).Query()
	if err != nil {
		return nil, nil, errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()

	tags := NewStatsTree()
	sources := NewStatsTree()
	var tmp *StatsTree
	for rows.Next() {
		var k string
		var id int64
		var parentID null.Int64
		var cuIDs pq.Int64Array
		err = rows.Scan(&id, &parentID, &k, &cuIDs)
		if err != nil {
			return nil, nil, errors.Wrap(err, "rows.Scan")
		}

		if k[0] == 't' {
			tmp = tags
		} else if k[0] == 's' {
			tmp = sources
		}

		tmp.insert(id, parentID.Int64, k[1:], cuIDs)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, errors.Wrap(err, "rows.Err()")
	}

	tags.accumulate()
	sources.accumulate()

	// blend in authors
	authors, err := mdbmodels.Authors(db, qm.Load("Sources")).All()
	if err != nil {
		return nil, nil, errors.Wrap(err, "fetch authors")
	}
	for i := range authors {
		author := authors[i]
		node := new(StatsNode)
		node.uid = author.Code
		node.cuIDs = make(IDSet)
		sources.byID[-1*(author.ID+1)] = node
		for j := range author.R.Sources {
			node.cuIDs.Merge(sources.byID[author.R.Sources[j].ID].cuIDs)
		}
	}

	return tags.flatten(), sources.flatten(), nil
}
