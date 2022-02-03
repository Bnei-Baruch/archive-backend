package cache

import (
	"fmt"

	"github.com/Bnei-Baruch/archive-backend/utils"
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
	byID  map[int64]*StatsNode
	byUID map[string]*StatsNode
}

func (st *StatsTree) GetUniqueChildren(rootUIDs []string) ([]string, []int64) {
	uids, ids := st.GetChildren(rootUIDs)
	return utils.ClearDuplicateString(uids), utils.ClearDuplicateInt64(ids)
}
func (st *StatsTree) GetChildren(rootUIDs []string) ([]string, []int64) {
	chs := make([]*StatsNode, 0)
	for _, rootUID := range rootUIDs {
		root := st.byUID[rootUID]
		chs = append(chs, st.getAllChildren(root)...)
	}
	uids := make([]string, len(chs))
	ids := make([]int64, len(chs))
	for i, ch := range chs {
		uids[i] = ch.uid
		ids[i] = ch.id
	}
	return uids, ids
}

func (st *StatsTree) getAllChildren(root *StatsNode) []*StatsNode {
	if root == nil {
		return make([]*StatsNode, 0)
	}
	result := []*StatsNode{root}
	if root.children == nil {
		return result
	}
	for _, id := range root.children {
		ch := st.byID[id]
		result = append(result, st.getAllChildren(ch)...)
	}
	return result
}

func NewStatsTree() *StatsTree {
	st := new(StatsTree)
	st.byID = make(map[int64]*StatsNode)
	st.byUID = make(map[string]*StatsNode)
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

		st.byUID[v.uid] = v
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
		// merge parents histograms and collect next generation
		parents := make(map[int64]bool)
		for i := range s {
			node := st.byID[s[i]]
			if node.parentID != 0 {
				p := st.byID[node.parentID] // get parent
				parents[p.id] = true        // add to next gen of
				p.hist.Merge(node.hist)     // merge parent histogram with that of child
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
}

// flatten return a flat uid => Histogram lookup table.
// It's usually the only interesting result to use
// as the tree structure is not really needed once accumulated.
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
