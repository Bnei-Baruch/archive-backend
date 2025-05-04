package cache

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Bnei-Baruch/archive-backend/es"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
)

type SearchStatsCache interface {
	Provider
	IsTagWithUnits(uid string, cts ...string) bool
	IsTagWithEnoughUnits(uid string, count int, cts ...string) bool
	IsSourceWithUnits(uid string, cts ...string) bool
	IsSourceWithEnoughUnits(uid string, count int, cts ...string) bool
	IsTagHasSingleUnit(uid string, cts ...string) bool
	IsSourceHasSingleUnit(uid string, cts ...string) bool

	// |location| can be of: "Moscow" or "Russia|Moscow" or "Russia" or "" (empty for year constrain only)
	// |year| is 4 digit year string, e.g., "1998", "2010" or "" (empty for location constrain only)
	DoesConventionExist(location string, year string) bool
	DoesConventionSingle(location string, year string) bool
	// |holiday| is the UID of the tag that is children of 'holidays' tag
	DoesHolidayExist(holiday string, year string) bool
	DoesHolidaySingle(holiday string, year string) bool

	// Some of the sources (consts.NOT_TO_INCLUDE_IN_SOURCE_BY_POSITION) are restricted from these functions so you should not use them for general porpuses.
	GetSourceByPositionAndParent(parent string, position string, sourceTypeIds []int64) *string
	GetSourceParentAndPosition(source string, getSourceTypeIds bool) (*string, *string, []int64, error)

	GetProgramByCollectionAndPosition(collection_uid string, position string) *string
	GetCollectionRecentUnit(collection_uid string) *string
	IsContentUnitTypeArticle(uid string) bool
}

type SearchStatsCacheImpl struct {
	mdb                             *sql.DB
	tags                            ClassByTypeStats
	sources                         ClassByTypeStats
	conventions                     map[string]map[string]int
	holidayYears                    map[string]map[string]int
	sourcesByPositionAndParent      map[string]string
	programsByCollectionAndPosition map[string]string
	recentUnitsOfCollections        map[string]string
	unitsThatAreArticles            map[string]bool
}

func NewSearchStatsCacheImpl(mdb *sql.DB, sources, tags ClassByTypeStats) SearchStatsCache {
	ssc := new(SearchStatsCacheImpl)
	ssc.mdb = mdb
	ssc.sources = sources
	ssc.tags = tags
	return ssc
}

func (ssc *SearchStatsCacheImpl) String() string {
	return "SearchStatsCacheImpl"
}

func (ssc *SearchStatsCacheImpl) DoesHolidayExist(holiday string, year string) bool {
	return ssc.holidayYears[holiday][year] > 0
}

func (ssc *SearchStatsCacheImpl) DoesHolidaySingle(holiday string, year string) bool {
	//fmt.Printf("Holidays count for %s %s - %d\n", holiday, year, search.holidayYears[holiday][year])
	return ssc.holidayYears[holiday][year] == 1
}

func (ssc *SearchStatsCacheImpl) DoesConventionExist(location string, year string) bool {
	return ssc.conventions[year][location] > 0
}

func (ssc *SearchStatsCacheImpl) DoesConventionSingle(location string, year string) bool {
	//fmt.Printf("Conventions count for %s %s - %d\n", location, year, search.conventions[year][location])
	return ssc.conventions[year][location] == 1
}

func (ssc *SearchStatsCacheImpl) IsTagWithUnits(uid string, cts ...string) bool {
	return ssc.IsTagWithEnoughUnits(uid, 1, cts...)
}

func (ssc *SearchStatsCacheImpl) IsTagWithEnoughUnits(uid string, count int, cts ...string) bool {
	return ssc.isClassWithUnits("tags", uid, &count, nil, cts...)
}

func (ssc *SearchStatsCacheImpl) IsSourceWithUnits(uid string, cts ...string) bool {
	return ssc.IsSourceWithEnoughUnits(uid, 1, cts...)
}

func (ssc *SearchStatsCacheImpl) IsSourceWithEnoughUnits(uid string, count int, cts ...string) bool {
	return ssc.isClassWithUnits("sources", uid, &count, nil, cts...)
}

func (ssc *SearchStatsCacheImpl) IsTagHasSingleUnit(uid string, cts ...string) bool {
	one := 1
	return ssc.isClassWithUnits("tags", uid, &one, &one, cts...)
}

func (ssc *SearchStatsCacheImpl) IsSourceHasSingleUnit(uid string, cts ...string) bool {
	one := 1
	return ssc.isClassWithUnits("sources", uid, &one, &one, cts...)
}

func (ssc *SearchStatsCacheImpl) GetSourceByPositionAndParent(parent string, position string, sourceTypeIds []int64) *string {
	if len(sourceTypeIds) == 0 {
		sourceTypeIds = consts.ALL_SRC_TYPES
	}
	for _, typeId := range sourceTypeIds {
		// Key structure: parent of the requested source (like book name) - position of the requested source child (like chapter or part number) - source type (book, volume, article, etc...)
		key := fmt.Sprintf("%v-%v-%v", parent, position, typeId)
		if src, ok := ssc.sourcesByPositionAndParent[key]; ok {
			return &src
		}
	}
	return nil
}

func (ssc *SearchStatsCacheImpl) GetSourceParentAndPosition(source string, getSourceTypeIds bool) (*string, *string, []int64, error) {
	var parent *string
	var position *string
	typeIds := []int64{}
	// If a common usage for this function is needed, it is better to optimize it by managing a reverse map.
	for k, v := range ssc.sourcesByPositionAndParent {
		if v == source {
			s := strings.Split(k, "-")
			if parent == nil {
				parent = &s[0]
			}
			if position == nil {
				position = &s[1]
			}
			typeIdStr := s[2]
			if !getSourceTypeIds {
				break
			}
			typeId, err := strconv.ParseInt(typeIdStr, 10, 64)
			if err != nil {
				return nil, nil, []int64{}, err
			}
			typeIds = append(typeIds, typeId)
		}
	}
	return parent, position, typeIds, nil
}

func (ssc *SearchStatsCacheImpl) GetProgramByCollectionAndPosition(collection_uid string, position string) *string {
	key := fmt.Sprintf("%v-%v", collection_uid, position)
	if program_uid, ok := ssc.programsByCollectionAndPosition[key]; ok {
		return &program_uid
	}
	return nil
}

func (ssc *SearchStatsCacheImpl) GetCollectionRecentUnit(collection_uid string) *string {
	if unit_uid, ok := ssc.recentUnitsOfCollections[collection_uid]; ok {
		return &unit_uid
	}
	return nil
}

func (ssc *SearchStatsCacheImpl) IsContentUnitTypeArticle(uid string) bool {
	value, exists := ssc.unitsThatAreArticles[uid]
	return exists && value
}

func (ssc *SearchStatsCacheImpl) isClassWithUnits(class, uid string, minCount *int, maxCount *int, cts ...string) bool {
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
			if c, ok := h[cts[i]]; ok {
				minOk := minCount == nil || c >= *minCount
				maxOk := maxCount == nil || c <= *maxCount
				if minOk && maxOk {
					return true
				}
			}
		}
	}

	return false
}

func (ssc *SearchStatsCacheImpl) Refresh() error {
	var err error

	ssc.conventions, err = ssc.refreshConventions()
	if err != nil {
		return errors.Wrap(err, "Load conventions stats.")
	}
	ssc.holidayYears, err = ssc.refreshHolidayYears()
	if err != nil {
		return errors.Wrap(err, "Load holidays stats.")
	}
	ssc.sourcesByPositionAndParent, err = ssc.loadSourcesByPositionAndParent()
	if err != nil {
		return errors.Wrap(err, "Load source position map.")
	}
	ssc.programsByCollectionAndPosition, err = ssc.loadProgramsByCollectionAndPosition()
	if err != nil {
		return errors.Wrap(err, "Load program position map.")
	}
	ssc.recentUnitsOfCollections, err = ssc.loadCollectionsRecentUnit()
	if err != nil {
		return errors.Wrap(err, "Load collections first unit map.")
	}
	ssc.unitsThatAreArticles, err = ssc.loadUnitsThatAreArticles()
	if err != nil {
		return errors.Wrap(err, "Load units that are articles.")
	}
	return nil
}

func (ssc *SearchStatsCacheImpl) refreshHolidayYears() (map[string]map[string]int, error) {
	ret := make(map[string]map[string]int)

	// Replace || operator to & (intersect arrays) after upgrading Postgres to v.12
	rows, err := queries.Raw(`select t.uid as tag_uid, 
	array_remove(array_agg(distinct extract(year from (c.properties ->> 'start_date')::date)) || 
						  array_agg(distinct extract(year from (c.properties ->> 'end_date')::date)), NULL) as years		
	from tags t 
	join collections c on c.properties ->> 'holiday_tag' = t.uid
	where c.secure = 0 and c.published = true
	group by t.uid;`).Query(ssc.mdb)

	if err != nil {
		return nil, errors.Wrap(err, "refreshHolidays - Query failed.")
	}
	defer rows.Close()

	ret[""] = make(map[string]int) // Year without specific holiday
	for rows.Next() {
		var tagUID string
		var years pq.StringArray
		err := rows.Scan(&tagUID, &years)
		if err != nil {
			return nil, errors.Wrap(err, "refreshHolidays rows.Scan")
		}
		years = es.Unique(years) // Remove this after upgrading to Postgres 12 and changing the query above
		if _, ok := ret[tagUID]; !ok {
			ret[tagUID] = make(map[string]int)
		}
		for _, year := range years {
			ret[tagUID][""]++
			ret[""][year]++
			ret[tagUID][year]++
		}
	}

	return ret, nil
}

func (ssc *SearchStatsCacheImpl) refreshConventions() (map[string]map[string]int, error) {
	ret := make(map[string]map[string]int)
	var collections []*mdbmodels.Collection
	if err := mdbmodels.NewQuery(
		qm.From("collections as c"),
		qm.Where(fmt.Sprintf("c.type_id = %d and c.secure = 0 and c.published = true", mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_CONGRESS].ID))).
		Bind(nil, ssc.mdb, &collections); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	for _, c := range collections {
		// Accumulate convention by year and by location
		var props map[string]interface{}
		err := json.Unmarshal(c.Properties.JSON, &props)
		if err != nil {
			errors.Wrap(err, "Error reading collection convention properties.")
			continue
		}
		city := props["city"].(string)
		country := props["country"].(string)
		years := []string{""}
		var start_year string
		var end_year string
		if start_date := props["start_date"]; len(start_date.(string)) >= 4 {
			start_year = start_date.(string)[0:4]
			years = append(years, start_year)
		}
		if end_date := props["end_date"]; len(end_date.(string)) >= 4 && (len(years) == 0 || years[0] != end_date.(string)[0:4]) {
			end_year = end_date.(string)[0:4]
			if end_year != start_year {
				years = append(years, end_year)
			}
		}
		for _, year := range years {
			if _, ok := ret[year]; !ok {
				ret[year] = make(map[string]int)
			}
			ret[year][""]++ // Without location, just [congresses 2005]
			ret[year][city]++
			ret[year][country]++
			if city != "" && country != "" {
				ret[year][fmt.Sprintf("%s|%s", country, city)]++
			}
		}
	}
	return ret, nil
}

func (ssc *SearchStatsCacheImpl) loadSourcesByPositionAndParent() (map[string]string, error) {
	queryMask := `select p.uid as parent_uid, c.uid as source_uid, c.position, c.type_id from sources p
	join sources c on c.parent_id = p.id
	where c.position is not null and p.uid not in (%s)`
	notToInclude := []string{}
	for _, s := range consts.NOT_TO_INCLUDE_IN_SOURCE_BY_POSITION {
		notToInclude = append(notToInclude, fmt.Sprintf("'%s'", s))
	}
	query := fmt.Sprintf(queryMask, strings.Join(notToInclude, ","))
	rows, err := queries.Raw(query).Query(ssc.mdb) // Authors are not part of the query.
	if err != nil {
		return nil, errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()
	ret := map[string]string{}
	for rows.Next() {
		var parent_uid string // uid of parent source
		var source_uid string // uid of child source
		var position int      // position of child source
		var type_id int64     // type of child source
		err = rows.Scan(&parent_uid, &source_uid, &position, &type_id)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		key := fmt.Sprintf("%v-%v-%v", parent_uid, position, type_id)
		ret[key] = source_uid
	}

	// The query is intended for creating relations between parent to grandchild division
	// (source to part while there is a volume in between) of the TES,
	// having UID 'xtKmrbb9'
	query = `select p.uid as parent_uid, gc.uid as source_uid, gc.type_id 
	from sources p
		join sources c on c.parent_id = p.id
		join sources gc on gc.parent_id = c.id
	where c.position is not null and p.uid = 'xtKmrbb9'
	order by c.position, gc.position`
	rows, err = queries.Raw(ssc.mdb, query).Query()
	if err != nil {
		return nil, errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()
	position := 0
	for rows.Next() {
		var parent_uid string // uid of parent source
		var source_uid string // uid of child source
		var type_id int64     // type of child source
		position++
		err = rows.Scan(&parent_uid, &source_uid, &type_id)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		key := fmt.Sprintf("%v-%v-%v", parent_uid, position, type_id)
		ret[key] = source_uid
	}

	return ret, nil
}

func (ssc *SearchStatsCacheImpl) loadProgramsByCollectionAndPosition() (map[string]string, error) {
	queryMask := `select c.uid as collection_uid, ccu.position, ccu.name, cu.uid as program_uid
		from collections c
		join collections_content_units ccu on c.id = ccu.collection_id
		join content_units cu on cu.id = ccu.content_unit_id
		where cu.published = true and cu.secure = 0
		and c.published = true and c.secure = 0 
		and c.type_id = %d
		and c.uid in ('%s','%s')`
	// The numeration of 'position' values is inconsistent for most programs so currently we able to handle only 2 program collections.
	// For 'new life' programs we handle the 'name' column as chapter number instead of 'position'.
	query := fmt.Sprintf(queryMask, mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_VIDEO_PROGRAM].ID,
		consts.PROGRAM_COLLECTION_HAPITARON, consts.PROGRAM_COLLECTION_NEW_LIFE)
	rows, err := queries.Raw(query).Query(ssc.mdb)
	if err != nil {
		return nil, errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()
	ret := map[string]string{}
	for rows.Next() {
		var collection_uid string
		var position int
		var name string
		var program_uid string
		err = rows.Scan(&collection_uid, &position, &name, &program_uid)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		if collection_uid == consts.PROGRAM_COLLECTION_NEW_LIFE {
			if val, err := strconv.Atoi(name); err == nil {
				position = val
			} else {
				fmt.Printf("The value '%v' of column 'name' from 'collections_content_units' was expected to be numeric and present the chapter number for New Life program.\n", name)
				continue
			}
		} else if collection_uid == consts.PROGRAM_COLLECTION_HAPITARON {
			position--
		}
		key := fmt.Sprintf("%v-%v", collection_uid, position)
		ret[key] = program_uid
	}
	return ret, nil
}

func (ssc *SearchStatsCacheImpl) loadCollectionsRecentUnit() (map[string]string, error) {
	query := `SELECT DISTINCT ON (c.id) c.uid AS collection_uid, cu.uid AS first_content_unit_uid
	FROM collections AS c
	JOIN collections_content_units AS ccu ON c.id = ccu.collection_id
	JOIN content_units AS cu ON ccu.content_unit_id = cu.id
	where c.secure = 0 and c.published = true
	and cu.secure = 0 and cu.published = true
	ORDER BY c.id, (cu.properties->>'film_date')::date DESC`
	rows, err := queries.Raw(query).Query(ssc.mdb)
	if err != nil {
		return nil, errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()
	ret := map[string]string{}
	for rows.Next() {
		var collection_uid string
		var first_content_unit_uid string
		err = rows.Scan(&collection_uid, &first_content_unit_uid)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		ret[collection_uid] = first_content_unit_uid
	}
	return ret, nil
}

func (ssc *SearchStatsCacheImpl) loadUnitsThatAreArticles() (map[string]bool, error) {
	queryMask := `select uid from content_units where type_id = %d and secure=0 and published=true`
	query := fmt.Sprintf(queryMask, mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_ARTICLE].ID)
	rows, err := queries.Raw(query).Query(ssc.mdb)
	if err != nil {
		return nil, errors.Wrap(err, "queries.Raw")
	}
	defer rows.Close()
	ret := map[string]bool{}
	for rows.Next() {
		var uid string
		err = rows.Scan(&uid)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		ret[uid] = true
	}
	return ret, nil
}
