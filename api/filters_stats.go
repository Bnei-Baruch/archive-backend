package api

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
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
	DB           *sql.DB
	TagTree      *cache.StatsTree
	Scope        string
	ScopeArgs    []interface{}
	Resp         *StatsClassResponse
	FetchOptions *StatsFetchOptions
}

const CITY_COUNTRY_SEPARATOR = "___"

func (fs *FilterStats) scan(q string) error {
	rows, err := queries.Raw(q, fs.ScopeArgs...).Query(fs.DB)
	if err != nil {
		return errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()

	tags := NewStatsTree()
	sources := NewStatsTree()
	var tmp *StatsTree
	byLang := make(map[string]int)
	byType := make(map[string]int)
	byC := make(map[string]int)
	byPerson := make(map[string]int)
	byMedia := make(map[string]int)
	byOLang := make(map[string]int)
	byLocation := make(map[string]CityItem)
	byDayPart := make(map[string]int)
	total := make(map[int64]bool)
	for rows.Next() {
		var k string
		var id int64
		var parentID null.Int64
		var ids pq.Int64Array
		var count int
		err = rows.Scan(&id, &parentID, &k, &ids, &count)
		if err != nil {
			return errors.Wrap(err, "rows.Scan")
		}
		for _, id := range ids {
			total[id] = true
		}
		prefix := k[:2]
		key := k[2:]

		switch prefix {
		case "tg":
			tmp = tags
		case "sr":
			tmp = sources
		case "lg":
			if _, ok := byLang[key]; !ok {
				byLang[key] = 0
			}
			byLang[key] += count
			continue
		case "ct":
			ct := mdb.CONTENT_TYPE_REGISTRY.ByID[id].Name
			byType[ct] = count
			continue
		case "cl":
			byC[key] = count
			continue
		case "pr":
			byPerson[key] = count
			continue
		case "mt":
			byMedia[key] = count
			continue
		case "ol":
			byOLang[key] = count
			continue
		case "co":
			spl := strings.Split(key, CITY_COUNTRY_SEPARATOR)
			byLocation[spl[0]] = CityItem{Count: count, City: spl[0], Country: spl[1]}
			continue
		case "dp":
			if key == "0" || key == "" {
				continue
			}
			byDayPart[key] = count
			continue
		default:
			continue
		}

		tmp.insert(id, parentID.Int64, k[2:], ids)
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "rows.Err()")
	}

	tags.accumulate()
	sources.accumulate()

	if fs.FetchOptions == nil || fs.FetchOptions.WithSources {
		// blend in authors
		authors, err := mdbmodels.Authors(qm.Load("Sources")).All(fs.DB)
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
	}
	fs.Resp.Tags = tags.flatten()
	fs.Resp.Sources = sources.flatten()
	fs.Resp.Languages = byLang
	fs.Resp.ContentTypes = byType
	fs.Resp.Collections = byC
	fs.Resp.Persons = byPerson
	fs.Resp.MediaTypes = byMedia
	fs.Resp.OriginalLanguages = byOLang
	fs.Resp.Locations = byLocation
	fs.Resp.DayPart = byDayPart

	fs.Resp.Total = int64(len(total))
	return nil
}

type FilterCUStats struct {
	FilterStats
}

func (fs *FilterCUStats) GetStats() error {
	qs := make([]string, 0)
	if fs.FetchOptions.WithSources {
		qs = append(qs, `
SELECT
  s.id,
  s.parent_id,
  concat('sr', s.uid),
  array_agg(distinct cus.content_unit_id),
	0
FROM sources s
  INNER JOIN content_units_sources cus on s.id = cus.source_id
  INNER JOIN fcu on cus.content_unit_id = fcu.id
GROUP BY s.id
UNION
SELECT
  s.id,
  s.parent_id,
  concat('sr', s.uid),
  '{}',
	0
FROM sources s
`,
		)
	}

	if fs.FetchOptions.WithTags {
		tids, err := getNotInTags(fs.TagTree, fs.DB)
		if err != nil {
			return err
		}

		qs = append(qs, fmt.Sprintf(`
SELECT
  t.id,
  t.parent_id,
  concat('tg', t.uid),
  array_agg(distinct cut.content_unit_id),
	0
FROM tags t
  INNER JOIN content_units_tags cut on t.id = cut.tag_id
  INNER JOIN fcu on cut.content_unit_id = fcu.id
  WHERE t.id NOT IN (%s)
GROUP BY t.id
UNION
SELECT
  t.id,
  t.parent_id,
  concat('tg', t.uid),
  '{}',
	0
FROM tags t
`, tids),
		)
	}

	if fs.FetchOptions.WithOriginalLanguages {
		qs = append(qs, `
SELECT
  0,
  NULL,
  concat('ol', cu.properties->>'original_language'),
  NULL,
  count(distinct cu.id)
FROM fcu cu
WHERE cu.properties->>'original_language' IS NOT NULL
GROUP BY cu.properties->>'original_language'
`,
		)
	}

	if fs.FetchOptions.WithLanguages {
		qs = append(qs, `
SELECT
  0,
  NULL,
  concat('lg', f.language),
  NULL,
  count(distinct f.content_unit_id)
FROM fcu
INNER JOIN files f ON f.content_unit_id = fcu.id
WHERE f.secure = 0 AND f.published IS TRUE AND f.language IS NOT NULL 
GROUP BY f.language
`,
		)
	}

	if fs.FetchOptions.WithMediaType {
		qs = append(qs, `
SELECT
  0,
  NULL,
  concat('mt', f.type),
  NULL,
  count(distinct f.content_unit_id)
FROM fcu
INNER JOIN files f ON f.content_unit_id = fcu.id
WHERE f.secure = 0 AND f.published IS TRUE AND f.type IS NOT NULL 
GROUP BY f.type
`,
		)
	}

	if fs.FetchOptions.WithContentTypes {
		qs = append(qs, `
SELECT
  fcu.type_id,
  NULL,
  concat('ct', fcu.type_id),
  NULL,
  count(distinct fcu.id)
FROM fcu
GROUP BY fcu.type_id
`)
	}

	if fs.FetchOptions.WithCollections {
		qs = append(qs, `
SELECT
  0,
  NULL,
  concat('cl', c.uid),
  NULL,
  count(distinct fcu.id)
FROM collections_content_units ccu
INNER JOIN fcu ON ccu.content_unit_id = fcu.id  
INNER JOIN collections c ON ccu.collection_id = c.id
GROUP BY c.uid
`)
	}

	if fs.FetchOptions.WithPersons {
		qs = append(qs, `
SELECT
  0,
  NULL,
  concat('pr', p.uid),
  NULL,
  count(distinct fcu.id)
FROM fcu
INNER JOIN content_units_persons cup ON fcu.id = cup.content_unit_id
INNER JOIN persons p ON cup.person_id = p.id
GROUP BY p.uid
`)
	}

	if fs.FetchOptions.WithPartOfDay {
		qs = append(qs, `
SELECT
  0,
  NULL,
  concat('dp', c.properties->>'number'),
  NULL,
  count(distinct fcu.id)
FROM collections_content_units ccu
INNER JOIN fcu ON ccu.content_unit_id = fcu.id  
INNER JOIN collections c ON ccu.collection_id = c.id
GROUP BY  c.properties->>'number'
`)
	}

	if len(qs) == 0 {
		return nil
	}
	qq := fmt.Sprintf("with fcu as (%s) %s", fs.Scope[:len(fs.Scope)-1], strings.Join(qs, " UNION "))
	return fs.scan(qq)
}

type FilterLabelStats struct {
	FilterStats
}

func (fs *FilterLabelStats) GetStats() error {
	qs := make([]string, 0)
	if fs.FetchOptions.WithSources {
		qs = append(qs, `SELECT
	  s.id,
	  s.parent_id,
	  concat('sr', s.uid),
	  array_agg(distinct fl.id),
		0
	FROM fl 
		INNER JOIN sources s ON s.uid = fl.suid
		GROUP BY s.id
	UNION
	SELECT
	  s.id,
	  s.parent_id,
	  concat('sr', s.uid),
	  '{}',
		0
	FROM sources s`)
	}

	if fs.FetchOptions.WithTags {
		tids, err := getNotInTags(fs.TagTree, fs.DB)
		if err != nil {
			return err
		}
		qs = append(qs, fmt.Sprintf(`
SELECT
  t.id,
  t.parent_id,
  concat('tg', t.uid),
  array_agg(distinct fl.id),
	0
FROM tags t
	INNER JOIN label_tag lt on t.id = lt.tag_id
	INNER JOIN fl on lt.label_id = fl.id
  	WHERE t.id NOT IN (%s)
	GROUP BY t.id
UNION
SELECT
  t.id,
  t.parent_id,
  concat('tg', t.uid),
  '{}',
	0
FROM tags t
`, tids),
		)
	}

	if fs.FetchOptions.WithLanguages {
		qs = append(qs, `SELECT
	  0,
	  NULL,
	  concat('lg', i18n.language),
	  NULL,
	  count(distinct fl.id)
	FROM label_i18n i18n
	INNER JOIN fl on i18n.label_id = fl.id
	GROUP BY i18n.language`)
	}

	if fs.FetchOptions.WithContentTypes {
		qs = append(qs, `SELECT
	  fl.type_id,
	  NULL,
	  concat('ct', fl.type_id),
	  NULL,
	  count(distinct fl.id)
	FROM fl
	GROUP BY fl.type_id`)
	}

	if len(qs) == 0 {
		return nil
	}
	qq := fmt.Sprintf("with fl as (%s) %s", fs.Scope[:len(fs.Scope)-1], strings.Join(qs, " UNION "))

	return fs.scan(qq)
}

type FilterCollectionStats struct {
	FilterStats
	*ContentTypesFilter
}

func (fs *FilterCollectionStats) GetStats() error {
	qs := make([]string, 0)
	if fs.FetchOptions.WithSources {
		qs = append(qs, `
SELECT
  s.id,
  s.parent_id,
  concat('sr', s.uid),
  array_agg(distinct c.id),
	0
FROM fc c
	INNER JOIN sources s ON s.uid = c.properties->>'source'
	GROUP BY s.id
UNION
SELECT
  s.id,
  s.parent_id,
  concat('sr', s.uid),
  '{}',
	0
FROM sources s
`,
		)
	}

	if fs.FetchOptions.WithTags {
		tids, err := getNotInTags(fs.TagTree, fs.DB)
		if err != nil {
			return err
		}
		qs = append(qs, fmt.Sprintf(`
SELECT
  t.id,
  t.parent_id,
  concat('tg', t.uid),
  array_agg(distinct c.id),
	0
FROM fc c
	INNER JOIN tags t  ON c.properties->'tags' ? t.uid 
  	WHERE t.id NOT IN (%s)
	GROUP BY t.id
UNION
SELECT
  t.id,
  t.parent_id,
  concat('tg', t.uid),
  '{}',
	0
FROM tags t
`, tids),
		)
	}

	if fs.FetchOptions.WithOriginalLanguages {
		qs = append(qs, `
SELECT
  0,
  NULL,
  concat('ol', c.properties->>'original_language'),
  NULL,
  count(distinct c.id)
FROM fc c
WHERE c.properties->>'original_language' IS NOT NULL
GROUP BY c.properties->>'original_language'
`,
		)
	}

	if fs.FetchOptions.WithLanguages {
		qs = append(qs, `
SELECT
  0,
  NULL,
  concat('lg', ccuf.lang),
  NULL,
  count(distinct c.id)
FROM fc as c
INNER JOIN (
	SELECT f.language as lang, ccu.collection_id as ccu_id  FROM collections_content_units ccu
	INNER JOIN content_units cu ON cu.id = ccu.content_unit_id
	INNER JOIN files f ON f.content_unit_id = cu.id
	WHERE (cu.secure=0 AND cu.published IS TRUE) AND (f.secure=0 AND f.published IS TRUE) AND f.language IS NOT NULL
) AS ccuf ON ccuf.ccu_id = c.id
WHERE ccuf.lang IS NOT NULL
GROUP BY lang
`,
		)
	}

	if fs.FetchOptions.WithContentTypes {
		qs = append(qs, `
SELECT
  c.type_id,
  NULL,
  concat('ct', c.type_id),
  NULL,
  count(distinct c.id)
FROM fc c
GROUP BY c.type_id
`,
		)
	}

	if fs.FetchOptions.WithLocations {
		qs = append(qs, fmt.Sprintf(`
SELECT
  0,
  NULL,
  concat('co', c.properties->>'city', '%s', c.properties->>'country'),
  NULL,
  count(distinct c.id)
FROM fc c
WHERE c.properties->>'country' IS NOT NULL OR  c.properties->>'city' IS NOT NULL 
GROUP BY  c.properties->>'city', c.properties->>'country'
`, CITY_COUNTRY_SEPARATOR),
		)
	}

	if fs.FetchOptions.WithPartOfDay {
		qs = append(qs, `
SELECT
  0,
  NULL,
  concat('dp', c.properties->>'number'),
  NULL,
  count(distinct c.id)
FROM fc c
GROUP BY  c.properties->>'number'
`)
	}

	if len(qs) == 0 {
		return nil
	}
	qq := fmt.Sprintf("with fc as (%s) %s", fs.Scope[:len(fs.Scope)-1], strings.Join(qs, " UNION "))
	return fs.scan(qq)
}

func getNotInTags(tree *cache.StatsTree, db *sql.DB) (string, error) {
	if _, tIds, err := GetNotInTagsIds(tree, db); err != nil {
		return "", err
	} else {
		return utils.JoinInt64(tIds, ","), nil
	}
}

func GetNotInTagsIds(tree *cache.StatsTree, db *sql.DB) ([]string, []int64, error) {
	t, err := mdbmodels.Tags(
		mdbmodels.TagWhere.Pattern.EQ(null.StringFrom("special-lesson")),
	).One(db)
	if err != nil {
		return nil, nil, err
	}
	tUids, tIds := tree.GetUniqueChildren([]string{t.UID})
	return tUids, tIds, nil
}
