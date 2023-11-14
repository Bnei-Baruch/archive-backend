package api

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/search"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var programsFallbackContentUnitTypes = []string{
	consts.CT_VIDEO_PROGRAM_CHAPTER,
	consts.CT_CLIP,
}

func MobileProgramsPageHandler(c *gin.Context) {
	var r MobileProgramsPageRequest
	if c.Bind(&r) != nil {
		return
	}

	cm := c.MustGet("CACHE").(cache.CacheManager)
	db := c.MustGet("MDB_DB").(*sql.DB)

	if len(r.ContentTypesFilter.ContentTypes) == 0 {
		r.ContentTypesFilter.ContentTypes = programsFallbackContentUnitTypes
	}

	cuRequest := ContentUnitsRequest{
		ListRequest:        r.ListRequest,
		ContentTypesFilter: r.ContentTypesFilter,
		SourcesFilter:      r.SourcesFilter,
		TagsFilter:         r.TagsFilter,
		PersonsFilter:      r.PersonsFilter,
	}

	resp, err := handleContentUnits(cm, db, cuRequest)

	result := &MobileContentUnitResponse{
		ListResponse: resp.ListResponse,
		Items:        make([]*MobileContentUnitResponseItem, 0, len(resp.ContentUnits)),
	}

	var contentUnitUids []string
	itemsMap := make(map[string]*MobileContentUnitResponseItem)
	imagesUrlTemplate := viper.GetString("content_unit_images.url_template")
	for _, pItem := range resp.ContentUnits {
		var date *time.Time
		if pItem.FilmDate != nil {
			date = &pItem.FilmDate.Time
		}
		var collection *Collection
		for _, col := range pItem.Collections {
			collection = col
			break
		}

		duration := int64(pItem.Duration)
		item := &MobileContentUnitResponseItem{
			ContentUnitUid: pItem.ID,
			CollectionId:   &collection.ID,
			Title:          pItem.Name,
			Description:    collection.Name,
			Date:           date,
			ContentType:    pItem.ContentType,
			Duration:       &duration,
			Image:          fmt.Sprintf(imagesUrlTemplate, pItem.ID),
		}

		contentUnitUids = append(contentUnitUids, item.ContentUnitUid)
		itemsMap[item.ContentUnitUid] = item
		result.Items = append(result.Items, item)
	}

	mapViewsToMobileResponseItems[*MobileContentUnitResponseItem](contentUnitUids, itemsMap)
	concludeRequest(c, result, err)
}

type responseItemType interface {
	*MobileContentUnitResponseItem | *MobileSearchResponseItem | *MobileFeedResponseItem
	SetViews(views *int64)
}

func mapViewsToMobileResponseItems[T responseItemType](contentUnitUids []string, itemsMap map[string]T) {
	if viewsResp, err := getViewsByCUIds(contentUnitUids); err != nil {
		log.Error(err.Error())
	} else {
		for ix := range viewsResp.Views {
			viewsCount := viewsResp.Views[ix]
			uid := contentUnitUids[ix]
			item := itemsMap[uid]
			item.SetViews(&viewsCount)
		}
	}
}

func LessonOverviewHandler(c *gin.Context) {
	var request LessonOverviewRequest
	if c.Bind(&request) != nil {
		return
	}
	db := c.MustGet("MDB_DB").(*sql.DB)
	cm := c.MustGet("CACHE").(cache.CacheManager)
	resp, err := getLessonOverviewsPage(cm, db, request)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	var collectionIds []int64
	var cuIds []int64
	var contentUnitUids []string
	itemsMap := make(map[string]*MobileContentUnitResponseItem)
	for _, item := range resp.Items {
		itemsMap[item.ContentUnitUid] = item
		if item.internalCollectionId != nil {
			collectionIds = append(collectionIds, *item.internalCollectionId)
		}

		cuIds = append(cuIds, item.internalUnitId)
		contentUnitUids = append(contentUnitUids, item.ContentUnitUid)
	}

	if err = setI18ColNameDesc(db, request.BaseRequest, collectionIds, cuIds, resp.Items); err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	mapViewsToMobileResponseItems[*MobileContentUnitResponseItem](contentUnitUids, itemsMap)
	concludeRequest(c, resp, nil)
}

var fallbackLessonsContentUnitTypes = []string{
	consts.CT_LESSON_PART,
	consts.CT_VIRTUAL_LESSON,
	consts.CT_WOMEN_LESSON,
	consts.CT_LECTURE,
	consts.CT_LESSONS_SERIES,
	consts.CT_DAILY_LESSON,
}

func getLessonOverviewsPage(cm cache.CacheManager, db *sql.DB, r LessonOverviewRequest) (*MobileContentUnitResponse, error) {
	//append collection filters
	cMods := []qm.QueryMod{SECURE_PUBLISHED_MOD}
	if len(r.ContentTypesFilter.ContentTypes) == 0 {
		r.ContentTypesFilter.ContentTypes = fallbackLessonsContentUnitTypes
	}

	if err := mobileLessonsAddCMods(cm, db, r, &cMods); err != nil {
		return nil, err
	}

	//append content units filters
	cuMods := []qm.QueryMod{SECURE_PUBLISHED_MOD_CU_PREFIX}
	if err := appendNotForDisplayCU(&cuMods); err != nil {
		return nil, err
	}
	if err := appendContentTypesFilterMods(&cuMods, r.ContentTypesFilter); err != nil {
		return nil, err
	}
	if err := appendDateRangeFilterMods(&cuMods, r.DateRangeFilter); err != nil {
		return nil, err
	}

	if err := appendSourcesFilterMods(cm, &cuMods, r.SourcesFilter); err != nil {
		return nil, err
	}
	appendTagsFilterMods(cm, &cuMods, r.TagsFilter)

	if err := appendMediaLanguageFilterMods(db, &cuMods, r.MediaLanguageFilter); err != nil {
		return nil, err
	}

	if err := appendMediaTypeFilterMods(&cuMods, r.MediaTypeFilter, true); err != nil {
		return nil, err
	} else if len(r.MediaType) > 0 {
		cMods = append(cMods, qm.Where("id < 0"))
	}

	if err := appendCollectionsFilterMods(db, &cuMods, r.CollectionsFilter); err != nil {
		return nil, err
	}
	if err := appendPersonsFilterMods(db, &cuMods, r.PersonsFilter); err != nil {
		return nil, err
	}
	if err := appendOriginalLanguageFilterMods(&cuMods, r.OriginalLanguageFilter, mdbmodels.TableNames.ContentUnits); err != nil {
		return nil, err
	}

	cCountQuery, cArgs := queries.BuildQuery(mdbmodels.Collections(append(cMods, qm.Select(`COUNT(DISTINCT "collections".id) AS count`))...).Query)
	cuCountQuery, cuArgs := queries.BuildQuery(mdbmodels.ContentUnits(append(cuMods, qm.Select(`*`))...).Query)
	cuCountQuery = startQueryArgCountFrom(cuCountQuery, len(cArgs))
	countQueryStr := `WITH
cCount AS (%s),
cuCount AS (
	SELECT COUNT(DISTINCT "cu".id) AS count
	FROM (%s) cu
	LEFT JOIN collections_content_units ccu ON cu.id = ccu.content_unit_id
	WHERE ccu IS NOT NULL
)
SELECT cc.count + cu.count FROM cCount cc, cuCount cu`
	countQueryJoint := fmt.Sprintf(countQueryStr, cCountQuery[:len(cCountQuery)-1], cuCountQuery[:len(cuCountQuery)-1])
	var cTotal int64
	countArgs := append(cArgs, cuArgs...)
	if err := queries.Raw(countQueryJoint, countArgs...).QueryRow(db).Scan(&cTotal); err != nil {
		return nil, err
	}

	if cTotal == 0 {
		return NewEmptyLessonOverviewResponse(), nil
	}

	cMods = append(cMods, qm.Select(`
			DISTINCT ON (id)
			coalesce((properties->>'start_date')::date, (properties->>'end_date')::date, (properties->>'film_date')::date, created_at) as date,
			id,
			uid,
            type_id,
			(properties ->> 'number')                      			 as number,
			(properties ->> 'start_date')::date                      as start_date,
			(properties ->> 'end_date')::date                        as end_date,
		    (properties -> 'tags' ->> 0)        					 as tag
		`))

	qc, args := queries.BuildQuery(mdbmodels.Collections(cMods...).Query)

	cuq, cuargs := queries.BuildQuery(mdbmodels.ContentUnits(append(cuMods, qm.Select(`*`))...).Query)
	cuq = startQueryArgCountFrom(cuq, len(args))

	cuqJoint := fmt.Sprintf(`
SELECT
	   cu.id,
	   cu.uid,
	   cu.type_id,
	   cu.properties,
	   cu.created_at,
       NULL AS tag,
       NULL AS collection_id,
	   NULL AS collection_uid,
	   NULL AS number,
	   coalesce((cu.properties ->> 'film_date')::date, cu.created_at) AS date,
	   NULL AS start_date,
	   NULL AS end_date
	FROM (%s) cu
	LEFT JOIN collections_content_units ccu ON cu.id = ccu.content_unit_id
	WHERE ccu IS NOT NULL
`, cuq[:len(cuq)-1])

	q := fmt.Sprintf(`
			WITH
				collecs AS (%s),
     			cusCollectionful AS (
						SELECT
								coalesce((ARRAY_AGG(cu.id ORDER BY ccu.position) FILTER (WHERE cll.tag IS NOT NULL))[1],
                                          (ARRAY_AGG(cu.id ORDER BY ccu.position))[1])                            AS id,
                                 coalesce((ARRAY_AGG(cu.uid ORDER BY ccu.position) FILTER (WHERE cll.tag IS NOT NULL))[1],
                                          (ARRAY_AGG(cu.uid ORDER BY ccu.position))[1])                           AS uid,
                                 cll.type_id,
--                                  coalesce((ARRAY_AGG(cu.type_id ORDER BY ccu.position) FILTER (WHERE cll.tag IS NOT NULL))[1],
--                                           (ARRAY_AGG(cu.type_id ORDER BY ccu.position))[1])                       AS type_id,
                                 coalesce((ARRAY_AGG(cu.properties ORDER BY ccu.position) FILTER (WHERE cll.tag IS NOT NULL))[1],
                                          (ARRAY_AGG(cu.properties ORDER BY ccu.position))[1])                    AS properties,
                                 coalesce((ARRAY_AGG(cu.created_at ORDER BY ccu.position) FILTER (WHERE cll.tag IS NOT NULL))[1],
                                          (ARRAY_AGG(cu.created_at ORDER BY ccu.position))[1])                    AS created_at,
                                 (ARRAY_AGG(cll.tag ORDER BY ccu.position) FILTER (WHERE cll.tag IS NOT NULL))[1] as tag,
							cll.id  as collection_id,
							cll.uid as collection_uid,
							cll.number,
							cll.date,
							cll.start_date,
							cll.end_date
             			FROM content_units cu
                      		JOIN collections_content_units ccu ON cu.id = ccu.content_unit_id
                      		JOIN collecs cll ON ccu.collection_id = cll.id
							WHERE (secure=0 AND published IS TRUE)
							group by cll.id, cll.uid, number, date, start_date, end_date, cll.type_id
             			-- ORDER BY ccu.collection_id, cu.created_at
				),
				cus AS (
					SELECT id, uid, type_id, properties, created_at, tag, collection_id, collection_uid, number, date, start_date, end_date
					-- On PostgreSQL 15 change the above line to:
					-- SELECT id::bigint, uid, type_id, properties, created_at, tag, collection_id::bigint, collection_uid, number, date, start_date::date, end_date::date
						FROM (%s) cu
					UNION
					(SELECT * FROM cusCollectionful)
				)
				SELECT
					c.id                                                         AS content_unit_id,
					c.uid                                                        AS content_unit_uid,
				    c.tag,
				    c.collection_id,
				    c.collection_uid,
				    c.type_id                                                    AS content_type,
				    0                                                            AS views,
				    c.date,
				    c.number,
				    c.start_date,
				    c.end_date,
				    c.properties ->> 'duration'                                  AS file_duration
				FROM cus c
				ORDER BY c.date DESC, c.id DESC LIMIT %d OFFSET %d
		`, qc[:len(qc)-1], cuqJoint[:len(cuqJoint)-1], r.PageSize, (r.PageNumber-1)*r.PageSize)

	rows, err := queries.Raw(q, append(args, cuargs...)...).Query(db)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resp := &MobileContentUnitResponse{
		ListResponse: ListResponse{
			Total: cTotal,
		},
		Items: make([]*MobileContentUnitResponseItem, 0),
	}

	imagesUrlTemplate := viper.GetString("content_unit_images.url_template")
	for rows.Next() {
		var contentUnitId int64
		var contentUnitUid string
		var collectionId *int64
		var collectionUid *string
		var tag *string
		var contentType int64
		var views *int64
		var number int
		var date *time.Time
		var startDate *time.Time
		var endDate *time.Time
		var duration int64

		err = rows.Scan(&contentUnitId, &contentUnitUid, &tag, &collectionId, &collectionUid, &contentType, &views,
			&date, &number, &startDate, &endDate, &duration)
		item := &MobileContentUnitResponseItem{
			ContentUnitUid:       contentUnitUid,
			CollectionId:         collectionUid,
			internalUnitId:       contentUnitId,
			internalCollectionId: collectionId,
			tag:                  tag,
			Image:                fmt.Sprintf(imagesUrlTemplate, contentUnitUid),
			Number:               number,
			Title:                "",
			Description:          "",
			ContentType:          mdb.CONTENT_TYPE_REGISTRY.ByID[contentType].Name,
			Date:                 date,
			StartDate:            startDate,
			EndDate:              endDate,
			Duration:             &duration,
			ViewsType: ViewsType{
				Views: views,
			},
		}

		resp.Items = append(resp.Items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return resp, nil
}

func mobileLessonsAddCMods(cm cache.CacheManager, db *sql.DB, r LessonOverviewRequest, cMods *[]qm.QueryMod) error {
	if err := appendContentTypesFilterMods(cMods, r.ContentTypesFilter); err != nil {
		return err
	}
	if err := appendIDsFilterMods(cMods, r.IDsFilter); err != nil {
		return NewBadRequestError(err)
	}
	if err := appendDateRangeCFilterMods(cMods, r.DateRangeFilter); err != nil {
		return NewBadRequestError(err)
	}
	if err := appendCollectionSourceFilterMods(cm, db, cMods, r.SourcesFilter); err != nil {
		return NewBadRequestError(err)
	}
	if err := appendCollectionTagsFilterMods(cm, db, cMods, r.TagsFilter); err != nil {
		return NewBadRequestError(err)
	}

	return nil
}

func setI18ColNameDesc(db *sql.DB, r BaseRequest, collectionIds []int64, cuIds []int64, items []*MobileContentUnitResponseItem) error {
	colNamesMap, err := loadCI18ns(db, r, collectionIds)
	if err != nil {
		return err
	}

	cuNamesMap, err := loadCUI18ns(db, r, cuIds)
	if err != nil {
		return err
	}

	languages := BaseRequestToContentLanguages(r)
	for _, ri := range items {
		if ri.internalCollectionId != nil && ri.tag != nil {
			i := 0
			for i < len(languages) && (ri.Title == "" || ri.Description == "") {
				if ri.internalCollectionId != (*int64)(nil) {
					if i18ns, ok := colNamesMap[*ri.internalCollectionId]; ok {
						li18n, ok := i18ns[languages[i]]
						if ok {
							if ri.Title == "" && li18n.Name.Valid && li18n.Name.String != "" {
								ri.Title = li18n.Name.String
							}
							if ri.Description == "" && li18n.Description.Valid && li18n.Description.String != "" {
								ri.Description = li18n.Description.String
							}
						}
					}
				}
				i++
			}
		}

		i := 0
		for i < len(languages) && (ri.Title == "" || ri.Description == "") {
			if i18ns, ok := cuNamesMap[ri.internalUnitId]; ok {
				li18n, ok := i18ns[languages[i]]
				if ok {
					if ri.Title == "" && li18n.Name.Valid && li18n.Name.String != "" {
						ri.Title = li18n.Name.String
					}
					if ri.Description == "" && li18n.Description.Valid && li18n.Description.String != "" {
						ri.Description = li18n.Description.String
					}
				}
			}
			i++
		}
	}

	return nil
}

func getViewsByCUIds(uIds []string) (*viewsResponse, error) {
	viewsUrl, err := getFeedApi("views")
	if err != nil {
		return nil, err
	}

	viewsPayload := map[string]interface{}{
		"uids": uIds,
	}

	viewsPayloadJson, err := json.Marshal(viewsPayload)
	if err != nil {
		return nil, err
	}

	viewsResp, err := http.Post(viewsUrl, "application/json", strings.NewReader(string(viewsPayloadJson)))
	if err != nil {
		return nil, err
	}

	viewsRespBytes, err := io.ReadAll(viewsResp.Body)
	if err != nil {
		return nil, err
	}

	views := new(viewsResponse)
	if err = json.Unmarshal(viewsRespBytes, views); err != nil {
		return nil, err
	}

	return views, nil
}

type viewsResponse struct {
	Views []int64 `json:"views"`
}

func getFeedApi(path string) (string, error) {
	baseUrl := viper.GetString("feed_service.url")
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	return url.JoinPath(baseUrl, path)
}

func MobileSearchHandler(c *gin.Context) {

	// Mobile search support all content types of the regular search beside:
	// 1. Article collections
	// 2. Blog posts
	// 3. Tweets
	// 4. Lesson series
	// 5. Landing pages

	log.Debugf("Mobile Language: %s", c.Query("language"))
	log.Infof("Mobile Query: [%s]", c.Query("q"))
	query := search.ParseQuery(c.Query("q"))
	query.Deb = false
	if c.Query("deb") == "true" {
		query.Deb = true
	}
	log.Infof("Parsed Query: %#v", query)
	if len(query.Term) == 0 && len(query.ExactTerms) == 0 {
		NewBadRequestError(errors.New("Can't search with no terms.")).Abort(c)
		return
	}

	var err error

	pageNoVal := 1
	pageNo := c.Query("page_no")
	if pageNo != "" {
		pageNoVal, err = strconv.Atoi(pageNo)
		if err != nil {
			NewBadRequestError(errors.New("page_no expects a positive number")).Abort(c)
			return
		}
	}

	size := consts.API_DEFAULT_PAGE_SIZE
	pageSize := c.Query("page_size")
	if pageSize != "" {
		size, err = strconv.Atoi(pageSize)
		if err != nil {
			NewBadRequestError(errors.New("page_size expects a positive number")).Abort(c)
			return
		}
		size = utils.Min(size, consts.API_MAX_PAGE_SIZE)
	}

	from := (pageNoVal - 1) * size

	sortByVal := consts.SORT_BY_RELEVANCE
	sortBy := c.Query("sort_by")
	if _, ok := consts.SORT_BY_VALUES[sortBy]; ok {
		sortByVal = sortBy
	}

	if len(query.Term) == 0 && len(query.ExactTerms) == 0 {
		sortByVal = consts.SORT_BY_SOURCE_FIRST
	}

	searchId := c.Query("search_id")
	suggestion := c.Query("suggest")

	// We use the MD5 of client IP as preference to resolve the "Bouncing Results" problem
	// see https://www.elastic.co/guide/en/elasticsearch/guide/current/_search_options.html
	preference := fmt.Sprintf("%x", md5.Sum([]byte(c.ClientIP())))

	esManager := c.MustGet("ES_MANAGER").(*search.ESManager)
	db := c.MustGet("MDB_DB").(*sql.DB)

	logger := c.MustGet("LOGGER").(*search.SearchLogger)
	cacheM := c.MustGet("CACHE").(cache.CacheManager)
	tc := c.MustGet("TOKENS_CACHE").(*search.TokensCache)
	variables := c.MustGet("VARIABLES").(search.VariablesV2)

	esc, err := esManager.GetClient()
	if err != nil {
		NewBadRequestError(errors.Wrap(err, "Failed to connect to ElasticSearch.")).Abort(c)
		return
	}

	// Detect input language
	detectQuery := strings.Join(append(query.ExactTerms, query.Term), " ")
	log.Debugf("Detect language input: (%s, %s, %s)", detectQuery, c.Query("language"), c.Request.Header.Get("Accept-Language"))
	// TBD check if app sends or need to send Accept-Language header
	query.LanguageOrder = utils.DetectLanguage(detectQuery, c.Query("language"), c.Request.Header.Get("Accept-Language"), nil)
	for k, v := range query.Filters {
		if k == consts.FILTER_MEDIA_LANGUAGE {
			addLang := true
			for _, flang := range v {
				for _, ilang := range query.LanguageOrder {
					if flang == ilang {
						// language already exist
						addLang = false
						break
					}
				}
				if addLang {
					query.LanguageOrder = append(query.LanguageOrder, flang)
				}
			}
			break
		}
	}
	// Quick workround to allow Spanish support when the interface language is Spanish (AS-99).
	if c.Query("language") == consts.LANG_SPANISH {
		for i, lang := range query.LanguageOrder {
			if lang == consts.LANG_SPANISH {
				query.LanguageOrder = append(query.LanguageOrder[:i], query.LanguageOrder[i+1:]...)
				break
			}
		}
		query.LanguageOrder = append([]string{consts.LANG_SPANISH}, query.LanguageOrder...)
	}

	// The logic up to this point is the same as the regular search handle

	se := search.NewESEngine(esc, db, cacheM, tc, variables, consts.ES_MOBILE_SEARCH_RESULT_TYPES)

	checkTypo := false // Currently not supported in mobile
	searchTweets := c.Query("search_tweets") == "true"
	searchLessonSeries := c.Query("search_lesson_series") == "true"

	res, err := se.DoSearch(
		context.TODO(),
		query,
		sortByVal,
		from,
		size,
		preference,
		checkTypo,
		searchTweets,
		searchLessonSeries,
		false, // Highlights are not currently supported in mobile
		time.Duration(0),
	)

	if err == nil {
		// TODO: How does this slows the search query? Consider logging in parallel.
		if !query.Deb {
			err = logger.LogSearch(query, sortByVal, from, size, searchId, suggestion, res, se.ExecutionTimeLog)
			if err != nil {
				log.Warnf("Error logging search: %+v %+v", err, res)
			}
		}

		mapIdsByType := map[string][]string{}
		mobileRespItemMap := map[string]*MobileSearchResponseItem{}
		allItems := []*MobileSearchResponseItem{}

		imagesUrlTemplate := viper.GetString("content_unit_images.url_template")

		for _, hit := range res.SearchResult.Hits.Hits {

			var result es.Result
			if hit.Source == nil {
				search.LogIfDeb(&query, fmt.Sprintf("Empty source in hit: %+v.", hit))
				continue
			}
			err = json.Unmarshal(*hit.Source, &result)
			if err != nil {
				search.LogIfDeb(&query, fmt.Sprintf("Unable to unmarshal source: %s. Error: %+v.", hit.Source, err))
				continue
			}
			var mobileResp *MobileSearchResponseItem
			var date *time.Time = nil

			if result.EffectiveDate != nil {
				date = &result.EffectiveDate.Time
			}
			if hit.Type == "result" {
				switch result.ResultType {
				case consts.ES_RESULT_TYPE_UNITS:
					var image *string
					var ct string
					isArticle := cacheM.SearchStats().IsContentUnitTypeArticle(result.MDB_UID)
					if isArticle {
						ct = consts.CT_ARTICLES
					} else {
						ct = result.ResultType
						imageStr := fmt.Sprintf(imagesUrlTemplate, result.MDB_UID)
						image = &imageStr
					}
					mobileResp = &MobileSearchResponseItem{
						ContentUnitUid: &result.MDB_UID,
						Image:          image,
						Title:          result.Title,
						Date:           date,
						Type:           ct,
					}

				case consts.ES_RESULT_TYPE_COLLECTIONS:
					var image *string
					firstUnit := cacheM.SearchStats().GetCollectionRecentUnit(result.MDB_UID)
					isArticle := firstUnit != nil && cacheM.SearchStats().IsContentUnitTypeArticle(*firstUnit)
					if isArticle {
						// Articles collection is not currently supported in mobile
						search.LogIfDeb(&query, "Skip result for mobile search: Articles Collection.")
						continue
					}
					if firstUnit != nil {
						// We retrieve collection image according to the first content unit of the collection.
						// Maybe we should retrieve collection image in a different way.
						imageStr := fmt.Sprintf(imagesUrlTemplate, *firstUnit)
						image = &imageStr
					}
					mobileResp = &MobileSearchResponseItem{
						CollectionUid:  &result.MDB_UID,
						Image:          image,
						Title:          result.Title,
						Date:           date,
						Type:           result.ResultType,
						ContentUnitUid: firstUnit,
					}

				case consts.ES_RESULT_TYPE_SOURCES:
					title := result.Title
					if len(result.FullTitle) > 0 {
						title = result.FullTitle
					}
					mobileResp = &MobileSearchResponseItem{
						SourceUid: &result.MDB_UID,
						Title:     title,
						Date:      date,
						Type:      result.ResultType,
					}

				default:
					search.LogIfDeb(&query, fmt.Sprintf("Skip result for mobile search: %s.", result.ResultType))
					continue
				}
			} else if hit.Type == consts.INTENT_HIT_TYPE_LESSONS {
				switch result.ResultType {
				case consts.ES_RESULT_TYPE_SOURCES:
					mobileResp = &MobileSearchResponseItem{
						SourceUid: &result.MDB_UID,
						Title:     result.Title,
						Type:      consts.SEARCH_RESULT_LESSONS_BY_SOURCE,
					}
				case consts.ES_RESULT_TYPE_TAGS:
					mobileResp = &MobileSearchResponseItem{
						TagUid: &result.MDB_UID,
						Title:  result.Title,
						Type:   consts.SEARCH_RESULT_LESSONS_BY_TAG,
					}
				default:
					search.LogIfDeb(&query, fmt.Sprintf("Skip result for mobile search: %s.", result.ResultType))
					continue
				}
			} else if hit.Type == consts.INTENT_HIT_TYPE_PROGRAMS {
				switch result.ResultType {
				case consts.ES_RESULT_TYPE_SOURCES:
					mobileResp = &MobileSearchResponseItem{
						SourceUid: &result.MDB_UID,
						Title:     result.Title,
						Type:      consts.SEARCH_RESULT_PROGRAMS_BY_SOURCE,
					}
				case consts.ES_RESULT_TYPE_TAGS:
					mobileResp = &MobileSearchResponseItem{
						TagUid: &result.MDB_UID,
						Title:  result.Title,
						Type:   consts.SEARCH_RESULT_PROGRAMS_BY_TAG,
					}
				default:
					search.LogIfDeb(&query, fmt.Sprintf("Skip result for mobile search: %s.", result.ResultType))
					continue
				}
			} else {
				search.LogIfDeb(&query, fmt.Sprintf("Skip hit for mobile search: %+v.", hit))
				continue
			}
			allItems = append(allItems, mobileResp)
			mobileRespItemMap[result.MDB_UID] = mobileResp
			mapIdsByType[result.ResultType] = append(mapIdsByType[result.ResultType], result.MDB_UID)
		}

		cuIds, exists := mapIdsByType[consts.ES_RESULT_TYPE_UNITS]
		if exists {
			mapViewsToMobileResponseItems[*MobileSearchResponseItem](cuIds, mobileRespItemMap)
		}

		mobileResponse := MobileSearchResponse{Total: res.SearchResult.Hits.TotalHits, Items: allItems}

		c.JSON(http.StatusOK, mobileResponse)

	} else {
		logErr := logger.LogSearchError(query, sortByVal, from, size, searchId, suggestion, err, se.ExecutionTimeLog)
		if logErr != nil {
			log.Warnf("Error logging search error: %+v %+v", logErr, err)
		}

		NewInternalError(err).Abort(c)
	}
}

func MobileFeed(c *gin.Context) {
	// get input json
	var r MobileFeedRequest
	if c.Bind(&r) != nil {
		return
	}

	feedInputJson, err := json.Marshal(r)
	if err != nil {
		NewInternalError(err).Abort(c)
	}

	feedApi, err := getFeedApi("feed")
	if err != nil {
		NewInternalError(err).Abort(c)
	}

	feedResponseObj, err := http.Post(feedApi, "application/json", strings.NewReader(string(feedInputJson)))
	if err != nil {
		NewInternalError(err).Abort(c)
	}

	// convert response body to byte arr and then get the feed array
	feedRespBytes, err := io.ReadAll(feedResponseObj.Body)
	if err != nil {
		log.Error(err.Error())
	}

	feedBody := new(feedResponseType)
	if err = json.Unmarshal(feedRespBytes, feedBody); err != nil || feedBody == nil {
		log.Error(err.Error())
	}

	imagesUrlTemplate := viper.GetString("content_unit_images.url_template")
	var mobilefeedResponse []*MobileFeedResponseItem

	db := c.MustGet("MDB_DB").(*sql.DB)
	language := c.Query("language")

	var cuIds []string
	itemsMap := make(map[string]*MobileFeedResponseItem)

	cuMap, err := getContentUnitsByLanguage(language, db)

	if err != nil {
		log.Error(err.Error())
	}

	for _, item := range feedBody.Feed {
		imageStr := fmt.Sprintf(imagesUrlTemplate, item.ContentUnitUid)

		feedResp := &MobileFeedResponseItem{
			ContentUnitUid: item.ContentUnitUid,
			Type:           item.ContentType,
			Image:          &imageStr,
			Date:           item.Date,
			Title:          cuMap[item.ContentUnitUid],
		}

		itemsMap[item.ContentUnitUid] = feedResp
		cuIds = append(cuIds, item.ContentUnitUid)
		mobilefeedResponse = append(mobilefeedResponse, feedResp)
	}

	// set views
	mapViewsToMobileResponseItems[*MobileFeedResponseItem](cuIds, itemsMap)

	c.JSON(http.StatusOK, mobilefeedResponse)
}

func getContentUnitsByLanguage(language string, db *sql.DB) (map[string]string, error) {
	QUERY_TITLE := `SELECT uid, name
									FROM content_unit_i18n i18n
									JOIN content_units cu ON cu.id = i18n.content_unit_id
									WHERE language = '%s'`

	rsql := fmt.Sprintf(QUERY_TITLE, language)
	rows, err := queries.Raw(rsql).Query(db)

	if err != nil {
		return nil, errors.Wrap(err, "getContentUnits queries.Raw(rsql).Query(db)")
	}

	defer rows.Close()

	cuMap := make(map[string]string) // contentUnits

	for rows.Next() {
		var uid string
		var name string
		err := rows.Scan(&uid, &name)
		if err != nil {
			return nil, errors.Wrap(err, "getContentUnits rows.Scan")
		}

		cuMap[uid] = name
	}

	return cuMap, nil
}

type feedResponseType struct {
	Feed  []MobileFeedItem `json:"feed"`
	Feeds interface{}      `json:"feeds"`
}
