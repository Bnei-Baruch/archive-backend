package api

import (
	"database/sql"
	"fmt"
	"github.com/Bnei-Baruch/archive-backend/mdb"

	"github.com/Bnei-Baruch/sqlboiler/queries"
	"github.com/lib/pq"
	"github.com/pkg/errors"
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
	ids      IDSet
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
				p.ids.Merge(node.ids)       // merge parent ID set with that of child
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
	root.ids = make(IDSet)
	st.byID[-1] = root
	for _, v := range st.byID {
		if v.parentID == 0 {
			root.ids.Merge(v.ids)
		}
	}
}

// flatten return a flat uid => Histogram lookup table.
// It's usually the only interesting result to use
// as the tree structure is not really needed once accumulated.
func (st *StatsTree) flatten() ClassificationStats {
	byUID := make(ClassificationStats, len(st.byID))
	for _, v := range st.byID {
		if count := len(v.ids); count > 0 {
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
		node.ids = make(IDSet)
		st.byID[id] = node
	}
	for i := range ids {
		node.ids.Increment(ids[i])
	}
}

type FilterStats struct {
	DB        *sql.DB
	Scope     string
	ScopeArgs []interface{}
	Resp      *StatsClassResponse
}

func (fs *FilterStats) scan(q string) error {
	rows, err := queries.Raw(fs.DB, q, fs.ScopeArgs...).Query()
	if err != nil {
		return errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()

	tags := NewStatsTree()
	sources := NewStatsTree()
	var tmp *StatsTree
	byLang := make(map[string]int)
	byType := make(map[string]int)
	total := make(map[int64]bool)
	for rows.Next() {
		var k string
		var id int64
		var parentID null.Int64
		var ids pq.Int64Array
		err = rows.Scan(&id, &parentID, &k, &ids)
		if err != nil {
			return errors.Wrap(err, "rows.Scan")
		}
		for _, id := range ids {
			total[id] = true
		}
		if k[0] == 't' {
			tmp = tags
		} else if k[0] == 's' {
			tmp = sources
		} else if k[0] == 'l' {
			byLang[k[1:]] = len(ids)
			continue
		} else if k[0] == 'c' {
			ct := mdb.CONTENT_TYPE_REGISTRY.ByID[id].Name
			byType[ct] = len(ids)
			continue
		}

		tmp.insert(id, parentID.Int64, k[1:], ids)
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "rows.Err()")
	}

	tags.accumulate()
	sources.accumulate()

	// blend in authors
	authors, err := mdbmodels.Authors(fs.DB, qm.Load("Sources")).All()
	if err != nil {
		return errors.Wrap(err, "fetch authors")
	}
	for i := range authors {
		author := authors[i]
		node := new(StatsNode)
		node.uid = author.Code
		node.ids = make(IDSet)
		sources.byID[-1*(author.ID+1)] = node
		for j := range author.R.Sources {
			node.ids.Merge(sources.byID[author.R.Sources[j].ID].ids)
		}
	}

	fs.Resp.Tags = tags.flatten()
	fs.Resp.Sources = sources.flatten()
	fs.Resp.Languages = byLang
	fs.Resp.ContentTypes = byType
	fs.Resp.Total = int64(len(total))
	return nil
}

type FilterCUStats struct {
	FilterStats
}

func (fs *FilterCUStats) GetStats() error {
	qq := fmt.Sprintf(`with fcu as (%s)
	SELECT
	  s.id,
	  s.parent_id,
	  concat('s', s.uid),
	  array_agg(distinct cus.content_unit_id)
	FROM sources s
	  INNER JOIN content_units_sources cus on s.id = cus.source_id
	  INNER JOIN fcu on cus.content_unit_id = fcu.id
	GROUP BY s.id
	UNION
	SELECT
	  s.id,
	  s.parent_id,
	  concat('s', s.uid),
	  '{}'
	FROM sources s
	UNION
	SELECT
	  t.id,
	  t.parent_id,
	  concat('t', t.uid),
	  array_agg(distinct cut.content_unit_id)
	FROM tags t
	  INNER JOIN content_units_tags cut on t.id = cut.tag_id
	  INNER JOIN fcu on cut.content_unit_id = fcu.id
	GROUP BY t.id
	UNION
	SELECT
	  t.id,
	  t.parent_id,
	  concat('t', t.uid),
	  '{}'
	FROM tags t
	UNION
	SELECT
	  0,
	  NULL,
	  concat('l', f.language),
	  array_agg(distinct f.content_unit_id)
	FROM files f
	INNER JOIN fcu on f.content_unit_id = fcu.id
	WHERE f.secure = 0 AND f.published IS TRUE
	GROUP BY f.language
	UNION
	SELECT
	  fcu.type_id,
	  NULL,
	  concat('c', fcu.type_id),
	  array_agg(distinct fcu.id)
	FROM fcu
	GROUP BY fcu.type_id
	`, fs.Scope[:len(fs.Scope)-1])
	return fs.scan(qq)
}

type FilterLabelStats struct {
	FilterStats
}

func (fs *FilterLabelStats) GetStats() error {
	qq := fmt.Sprintf(`with fl as (%s)
	SELECT
	  s.id,
	  s.parent_id,
	  concat('s', s.uid),
	  array_agg(distinct fl.id)
	FROM fl 
		INNER JOIN sources s ON s.uid = fl.suid
		GROUP BY s.id
	UNION
	SELECT
	  s.id,
	  s.parent_id,
	  concat('s', s.uid),
	  '{}'
	FROM sources s
	UNION
	SELECT
	  t.id,
	  t.parent_id,
	  concat('t', t.uid),
	  array_agg(distinct fl.id)
	FROM tags t
		INNER JOIN label_tag lt on t.id = lt.tag_id
		INNER JOIN fl on lt.label_id = fl.id
		GROUP BY t.id
	UNION
	SELECT
	  t.id,
	  t.parent_id,
	  concat('t', t.uid),
	  '{}'
	FROM tags t
	UNION
	SELECT
	  0,
	  NULL,
	  concat('l', i18n.language),
	  array_agg(distinct fl.id)
	FROM label_i18n i18n
	INNER JOIN fl on i18n.label_id = fl.id
	GROUP BY i18n.language
	UNION
	SELECT
	  fl.type_id,
	  NULL,
	  concat('c', fl.type_id),
	  array_agg(distinct fl.id)
	FROM fl
	GROUP BY fl.type_id
	`,
		fs.Scope[:len(fs.Scope)-1])
	return fs.scan(qq)
}
