package api

import (
	"context"
	"crypto/md5"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	log "github.com/Sirupsen/logrus"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"gopkg.in/gin-gonic/gin.v1"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/search"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var SECURE_PUBLISHED_MOD = qm.Where(fmt.Sprintf("secure=%d AND published IS TRUE", consts.SEC_PUBLIC))

func CollectionsHandler(c *gin.Context) {
	r := CollectionsRequest{
		WithUnits: true,
	}
	if c.Bind(&r) != nil {
		return
	}

	resp, err := handleCollections(c.MustGet("MDB_DB").(*sql.DB), r)
	concludeRequest(c, resp, err)
}

func CollectionHandler(c *gin.Context) {
	var r ItemRequest
	if c.Bind(&r) != nil {
		return
	}

	r.UID = c.Param("uid")

	resp, err := handleCollection(c.MustGet("MDB_DB").(*sql.DB), r)
	concludeRequest(c, resp, err)
}

func ContentUnitsHandler(c *gin.Context) {
	var r ContentUnitsRequest
	if c.Bind(&r) != nil {
		return
	}

	resp, err := handleContentUnits(c.MustGet("MDB_DB").(*sql.DB), r)
	concludeRequest(c, resp, err)
}

func ContentUnitHandler(c *gin.Context) {
	var r BaseRequest
	if c.Bind(&r) != nil {
		return
	}

	db := c.MustGet("MDB_DB").(*sql.DB)

	uid := c.Param("uid")
	cu, err := mdbmodels.ContentUnits(db,
		SECURE_PUBLISHED_MOD,
		qm.Where("uid = ?", uid),
		qm.Load("Sources",
			"Tags",
			"Publishers",
			"CollectionsContentUnits",
			"CollectionsContentUnits.Collection",
			"DerivedContentUnitDerivations",
			"DerivedContentUnitDerivations.Source",
			"DerivedContentUnitDerivations.Source.Publishers",
			"SourceContentUnitDerivations",
			"SourceContentUnitDerivations.Derived",
			"SourceContentUnitDerivations.Derived.Publishers")).
		One()
	if err != nil {
		if err == sql.ErrNoRows {
			NewNotFoundError().Abort(c)
			return
		} else {
			NewInternalError(err).Abort(c)
			return
		}
	}

	u, err := mdbToCU(cu)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	// Derived & Source content units
	cuidsMap := make(map[string]int64)

	u.SourceUnits = make(map[string]*ContentUnit)
	for _, cud := range cu.R.DerivedContentUnitDerivations {
		su := cud.R.Source
		if consts.SEC_PUBLIC == su.Secure && su.Published {
			scu, err := mdbToCU(su)
			if err != nil {
				NewInternalError(err).Abort(c)
				return
			}

			// publishers
			scu.Publishers = make([]string, len(su.R.Publishers))
			for i, x := range su.R.Publishers {
				scu.Publishers[i] = x.UID
			}

			// Dirty hack for unique mapping - needs to parse in client...
			key := fmt.Sprintf("%s____%s", su.UID, cud.Name)
			u.SourceUnits[key] = scu
			cuidsMap[key] = su.ID
		}
	}

	u.DerivedUnits = make(map[string]*ContentUnit)
	for _, cud := range cu.R.SourceContentUnitDerivations {
		du := cud.R.Derived
		if consts.SEC_PUBLIC == du.Secure && du.Published {
			dcu, err := mdbToCU(du)
			if err != nil {
				NewInternalError(err).Abort(c)
				return
			}

			// publishers
			dcu.Publishers = make([]string, len(du.R.Publishers))
			for i, x := range du.R.Publishers {
				dcu.Publishers[i] = x.UID
			}

			// Dirty hack for unique mapping - needs to parse in client...
			key := fmt.Sprintf("%s____%s", du.UID, cud.Name)
			u.DerivedUnits[key] = dcu
			cuidsMap[key] = du.ID
		}
	}

	cuids := make([]int64, 1)
	cuids[0] = cu.ID
	for _, v := range cuidsMap {
		cuids = append(cuids, v)
	}

	// content units i18n
	cui18nsMap, err := loadCUI18ns(db, r.Language, cuids)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	if i18ns, ok := cui18nsMap[cu.ID]; ok {
		setCUI18n(u, r.Language, i18ns)
	}
	for k, v := range u.DerivedUnits {
		if i18ns, ok := cui18nsMap[cuidsMap[k]]; ok {
			setCUI18n(v, r.Language, i18ns)
		}
	}
	for k, v := range u.SourceUnits {
		if i18ns, ok := cui18nsMap[cuidsMap[k]]; ok {
			setCUI18n(v, r.Language, i18ns)
		}
	}

	// files (all CUs)
	fileMap, err := loadCUFiles(db, cuids)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	if files, ok := fileMap[cu.ID]; ok {
		if err := setCUFiles(u, files); err != nil {
			NewInternalError(err).Abort(c)
			return
		}
	}
	for k, v := range u.DerivedUnits {
		if files, ok := fileMap[cuidsMap[k]]; ok {
			if err := setCUFiles(v, files); err != nil {
				NewInternalError(err).Abort(c)
				return
			}
		}
	}
	for k, v := range u.SourceUnits {
		if files, ok := fileMap[cuidsMap[k]]; ok {
			if err := setCUFiles(v, files); err != nil {
				NewInternalError(err).Abort(c)
				return
			}
		}
	}

	// collections
	u.Collections = make(map[string]*Collection)
	cidsMap := make(map[string]int64)
	for _, ccu := range cu.R.CollectionsContentUnits {
		if consts.SEC_PUBLIC == ccu.R.Collection.Secure && ccu.R.Collection.Published {
			cl := ccu.R.Collection

			cc, err := mdbToC(cl)
			if err != nil {
				NewInternalError(err).Abort(c)
				return
			}

			// Dirty hack for unique mapping - needs to parse in client...
			key := fmt.Sprintf("%s____%s", cl.UID, ccu.Name)
			u.Collections[key] = cc

			cidsMap[key] = cl.ID
		}
	}

	// collections - i18n
	cids := make([]int64, 0)
	for _, v := range cidsMap {
		cids = append(cids, v)
	}
	if len(cids) > 0 {
		ci18nsMap, err := loadCI18ns(db, r.Language, cids)
		if err != nil {
			NewInternalError(err).Abort(c)
			return
		}
		for k, v := range u.Collections {
			if i18ns, ok := ci18nsMap[cidsMap[k]]; ok {
				setCI18n(v, r.Language, i18ns)
			}
		}
	}

	// sources
	u.Sources = make([]string, len(cu.R.Sources))
	for i, x := range cu.R.Sources {
		u.Sources[i] = x.UID
	}

	// tags
	u.Tags = make([]string, len(cu.R.Tags))
	for i, x := range cu.R.Tags {
		u.Tags[i] = x.UID
	}

	// publishers
	u.Publishers = make([]string, len(cu.R.Publishers))
	for i, x := range cu.R.Publishers {
		u.Publishers[i] = x.UID
	}

	c.JSON(http.StatusOK, u)
}

func LessonsHandler(c *gin.Context) {
	var r LessonsRequest
	if c.Bind(&r) != nil {
		return
	}

	// We're either in full lessons mode or lesson parts mode based on
	// filters that apply only to lesson parts (content_units)

	if utils.IsEmpty(r.Authors) &&
		utils.IsEmpty(r.Sources) &&
		utils.IsEmpty(r.Tags) {
		if r.OrderBy == "" {
			r.OrderBy = "(properties->>'film_date')::date desc, (properties->>'number')::int desc, created_at desc"
		}
		cr := CollectionsRequest{
			ContentTypesFilter: ContentTypesFilter{
				ContentTypes: []string{consts.CT_DAILY_LESSON, consts.CT_SPECIAL_LESSON},
			},
			ListRequest:     r.ListRequest,
			DateRangeFilter: r.DateRangeFilter,
			WithUnits:       true,
		}
		resp, err := handleCollections(c.MustGet("MDB_DB").(*sql.DB), cr)
		concludeRequest(c, resp, err)
	} else {
		if r.OrderBy == "" {
			r.OrderBy = "(properties->>'film_date')::date desc, created_at desc"
		}
		cur := ContentUnitsRequest{
			ContentTypesFilter: ContentTypesFilter{
				ContentTypes: []string{consts.CT_LESSON_PART},
			},
			ListRequest:     r.ListRequest,
			DateRangeFilter: r.DateRangeFilter,
			SourcesFilter:   r.SourcesFilter,
			TagsFilter:      r.TagsFilter,
		}
		resp, err := handleContentUnits(c.MustGet("MDB_DB").(*sql.DB), cur)
		concludeRequest(c, resp, err)
	}
}

func PublishersHandler(c *gin.Context) {
	var r PublishersRequest
	if c.Bind(&r) != nil {
		return
	}

	resp, err := handlePublishers(c.MustGet("MDB_DB").(*sql.DB), r)
	concludeRequest(c, resp, err)
}

func IsTokenStart(i int, runes []rune, lastQuote rune) bool {
	return i == 0 && !unicode.IsSpace(runes[0]) ||
		(i > 0 && !unicode.IsSpace(runes[i]) && unicode.IsSpace(runes[i-1]))
}

func IsTokenEnd(i int, runes []rune, lastQuote rune, lastQuoteIdx int) bool {
	return i == len(runes)-1 ||
		(i < len(runes)-1 && unicode.IsSpace(runes[i+1]) &&
			(lastQuote == rune(0) || runes[i] == lastQuote && lastQuoteIdx >= 0 && lastQuoteIdx < i))
}

func IsRuneQuotationMark(r rune) bool {
	return unicode.In(r, unicode.Quotation_Mark) || r == rune(1523) || r == rune(1524)
}

// Tokenizes string to work with user friendly escapings of quotes (see tests).
func Tokenize(str string) []string {
	runes := []rune(str)
	start := -1
	lastQuote := rune(0)
	lastQuoteIdx := -1
	parts := 0
	var tokens []string
	for i, r := range runes {
		if start == -1 && IsTokenStart(i, runes, lastQuote) {
			start = i
		}
		if i == start && lastQuote == rune(0) && IsRuneQuotationMark(r) {
			lastQuote = r
			lastQuoteIdx = i
		}
		if start >= 0 && IsTokenEnd(i, runes, lastQuote, lastQuoteIdx) {
			tokens = append(tokens, string(runes[start:i+1]))
			lastQuote = rune(0)
			lastQuoteIdx = -1
			start = -1
			parts += 1
		}
	}

	return tokens
}

// Parses query and extracts terms and filters.
func ParseQuery(q string) search.Query {
	filters := make(map[string][]string)
	var terms []string
	var exactTerms []string
	for _, t := range Tokenize(q) {
		isFilter := false
		for filter := range consts.FILTERS {
			prefix := fmt.Sprintf("%s:", filter)
			if isFilter = strings.HasPrefix(t, prefix); isFilter {
				filters[consts.FILTERS[filter]] = strings.Split(strings.TrimPrefix(t, prefix), ",")
				break
			}
		}
		if !isFilter {
			// Not clear what kind of decoding is happening here, utf-8?!
			runes := []rune(t)
			for _, c := range runes {
				fmt.Printf("%04x %s\n", c, string(c))
			}
			if len(runes) >= 2 && IsRuneQuotationMark(runes[0]) && runes[0] == runes[len(runes)-1] {
				exactTerms = append(exactTerms, string(runes[1:len(runes)-1]))
			} else {
				terms = append(terms, t)
			}
		}
	}
	return search.Query{Term: strings.Join(terms, " "), ExactTerms: exactTerms, Filters: filters}
}

func SearchHandler(c *gin.Context) {
	log.Infof("Query: [%s]", c.Query("q"))
	query := ParseQuery(c.Query("q"))
	log.Infof("Parsed Query: %#v", query)
	if len(query.Term) == 0 && len(query.Filters) == 0 && len(query.ExactTerms) == 0 {
		NewBadRequestError(errors.New("Can't search with no terms and no filters.")).Abort(c)
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

	pageSizeVal := consts.API_DEFAULT_PAGE_SIZE
	pageSize := c.Query("page_size")
	if pageSize != "" {
		pageSizeVal, err = strconv.Atoi(pageSize)
		if err != nil {
			NewBadRequestError(errors.New("page_size expects a positive number")).Abort(c)
			return
		}
		pageSizeVal = utils.Min(pageSizeVal, consts.API_MAX_PAGE_SIZE)
	}

	sortByVal := consts.SORT_BY_RELEVANCE
	sortBy := c.Query("sort_by")
	if _, ok := consts.SORT_BY_VALUES[sortBy]; ok {
		sortByVal = sortBy
	}

	// We use the MD5 of client IP as preference to resolve the "Bouncing Results" problem
	// see https://www.elastic.co/guide/en/elasticsearch/guide/current/_search_options.html
	preference := fmt.Sprintf("%x", md5.Sum([]byte(c.ClientIP())))

	esc := c.MustGet("ES_CLIENT").(*elastic.Client)
	db := c.MustGet("MDB_DB").(*sql.DB)
	se := search.NewESEngine(esc, db)

	// Detect input language
	detectQuery := strings.Join(append(query.ExactTerms, query.Term), " ")
	log.Info("Detect language input:", detectQuery)
	query.LanguageOrder = utils.DetectLanguage(detectQuery, c.Query("language"), c.Request.Header.Get("Accept-Language"), nil)

	res, err := se.DoSearch(
		context.TODO(),
		query,
		sortByVal,
		(pageNoVal-1)*pageSizeVal,
		pageSizeVal,
		preference,
	)
	if err == nil {
		c.JSON(http.StatusOK, res)
	} else {
		NewInternalError(err).Abort(c)
	}
}

func AutocompleteHandler(c *gin.Context) {
	log.Infof("Query: [%s]", c.Query("q"))
	q := c.Query("q")
	if q == "" {
		NewBadRequestError(errors.New("Can't search for an empty term")).Abort(c)
		return
	}

	esc := c.MustGet("ES_CLIENT").(*elastic.Client)
	db := c.MustGet("MDB_DB").(*sql.DB)
	se := search.NewESEngine(esc, db)

	// Detect input language
	order := utils.DetectLanguage(q, c.Query("language"), c.Request.Header.Get("Accept-Language"), nil)

	// Have a 50ms deadline on the search engine call.
	// It's autocomplete after all...
	ctx, cancelFn := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelFn()

	res, err := se.GetSuggestions(ctx, search.Query{Term: q, LanguageOrder: order})
	if err == nil {
		c.JSON(http.StatusOK, res)
	} else {
		NewInternalError(err).Abort(c)
	}
}

func RecentlyUpdatedHandler(c *gin.Context) {
	resp, err := handleRecentlyUpdated(c.MustGet("MDB_DB").(*sql.DB))
	concludeRequest(c, resp, err)
}

func handleCollections(db *sql.DB, r CollectionsRequest) (*CollectionsResponse, *HttpError) {
	mods := []qm.QueryMod{SECURE_PUBLISHED_MOD}

	// filters
	if err := appendIDsFilterMods(&mods, r.IDsFilter); err != nil {
		return nil, NewBadRequestError(err)
	}
	if err := appendContentTypesFilterMods(&mods, r.ContentTypesFilter); err != nil {
		return nil, NewBadRequestError(err)
	}
	if err := appendDateRangeFilterMods(&mods, r.DateRangeFilter); err != nil {
		return nil, NewBadRequestError(err)
	}

	// count query
	var total int64
	countMods := append([]qm.QueryMod{qm.Select("count(DISTINCT id)")}, mods...)
	err := mdbmodels.Collections(db, countMods...).QueryRow().Scan(&total)
	if err != nil {
		return nil, NewInternalError(err)
	}
	if total == 0 {
		return NewCollectionsResponse(), nil
	}

	// order, limit, offset
	_, offset, err := appendListMods(&mods, r.ListRequest)
	if err != nil {
		return nil, NewBadRequestError(err)
	}
	if int64(offset) >= total {
		return NewCollectionsResponse(), nil
	}

	if r.WithUnits {
		// Eager loading
		mods = append(mods, qm.Load(
			"CollectionsContentUnits",
			"CollectionsContentUnits.ContentUnit"))
	}

	// data query
	collections, err := mdbmodels.Collections(db, mods...).All()
	if err != nil {
		return nil, NewInternalError(err)
	}

	// response - thin version
	if !r.WithUnits {
		cids := make([]int64, len(collections))
		for i, x := range collections {
			cids[i] = x.ID
		}

		ci18nsMap, err := loadCI18ns(db, r.Language, cids)
		if err != nil {
			return nil, NewInternalError(err)
		}

		// Response
		resp := &CollectionsResponse{
			ListResponse: ListResponse{Total: total},
			Collections:  make([]*Collection, len(collections)),
		}
		for i, x := range collections {
			c, err := mdbToC(x)
			if err != nil {
				return nil, NewInternalError(err)
			}
			if i18ns, ok := ci18nsMap[x.ID]; ok {
				setCI18n(c, r.Language, i18ns)
			}
			resp.Collections[i] = c
		}

		return resp, nil
	}

	// Response - thick version (with content units)

	// Filter secure & published content units
	// Load i18n for all collections and all units - total 2 DB round trips
	cids := make([]int64, len(collections))
	cuids := make([]int64, 0)
	for i, x := range collections {
		cids[i] = x.ID
		b := x.R.CollectionsContentUnits[:0]
		for _, y := range x.R.CollectionsContentUnits {
			if consts.SEC_PUBLIC == y.R.ContentUnit.Secure && y.R.ContentUnit.Published {
				b = append(b, y)
				cuids = append(cuids, y.ContentUnitID)
			}
			x.R.CollectionsContentUnits = b
		}
	}

	ci18nsMap, err := loadCI18ns(db, r.Language, cids)
	if err != nil {
		return nil, NewInternalError(err)
	}
	cui18nsMap, err := loadCUI18ns(db, r.Language, cuids)
	if err != nil {
		return nil, NewInternalError(err)
	}

	// Response
	resp := &CollectionsResponse{
		ListResponse: ListResponse{Total: total},
		Collections:  make([]*Collection, len(collections)),
	}
	for i, x := range collections {
		c, err := mdbToC(x)
		if err != nil {
			return nil, NewInternalError(err)
		}
		if i18ns, ok := ci18nsMap[x.ID]; ok {
			setCI18n(c, r.Language, i18ns)
		}

		// content units
		sort.SliceStable(x.R.CollectionsContentUnits, func(i int, j int) bool {
			return x.R.CollectionsContentUnits[i].Position < x.R.CollectionsContentUnits[j].Position
		})

		c.ContentUnits = make([]*ContentUnit, 0)
		for _, ccu := range x.R.CollectionsContentUnits {
			cu := ccu.R.ContentUnit

			u, err := mdbToCU(cu)
			if err != nil {
				return nil, NewInternalError(err)
			}
			if i18ns, ok := cui18nsMap[cu.ID]; ok {
				setCUI18n(u, r.Language, i18ns)
			}

			u.NameInCollection = ccu.Name
			c.ContentUnits = append(c.ContentUnits, u)
		}
		resp.Collections[i] = c
	}

	return resp, nil
}

func handleCollection(db *sql.DB, r ItemRequest) (*Collection, *HttpError) {

	c, err := mdbmodels.Collections(db,
		SECURE_PUBLISHED_MOD,
		qm.Where("uid = ?", r.UID),
		qm.Load("CollectionsContentUnits",
			"CollectionsContentUnits.ContentUnit")).
		One()
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NewNotFoundError()
		} else {
			return nil, NewInternalError(err)
		}
	}

	// collection
	cl, err := mdbToC(c)
	if err != nil {
		return nil, NewInternalError(err)
	}

	// collection i18n
	ci18nsMap, err := loadCI18ns(db, r.Language, []int64{c.ID})
	if err != nil {
		return nil, NewInternalError(err)
	}
	if i18ns, ok := ci18nsMap[c.ID]; ok {
		setCI18n(cl, r.Language, i18ns)
	}

	// content units
	cuids := make([]int64, 0)

	// filter secure & published
	b := c.R.CollectionsContentUnits[:0]
	for _, y := range c.R.CollectionsContentUnits {
		if consts.SEC_PUBLIC == y.R.ContentUnit.Secure && y.R.ContentUnit.Published {
			b = append(b, y)
			cuids = append(cuids, y.ContentUnitID)
		}
		c.R.CollectionsContentUnits = b
	}

	// load i18ns
	cui18nsMap, err := loadCUI18ns(db, r.Language, cuids)
	if err != nil {
		return nil, NewInternalError(err)
	}

	// sort CCUs
	sort.SliceStable(c.R.CollectionsContentUnits, func(i int, j int) bool {
		return c.R.CollectionsContentUnits[i].Position < c.R.CollectionsContentUnits[j].Position
	})

	// construct DTO's
	cl.ContentUnits = make([]*ContentUnit, 0)
	for _, ccu := range c.R.CollectionsContentUnits {
		cu := ccu.R.ContentUnit

		u, err := mdbToCU(cu)
		if err != nil {
			return nil, NewInternalError(err)
		}
		if i18ns, ok := cui18nsMap[cu.ID]; ok {
			setCUI18n(u, r.Language, i18ns)
		}

		u.NameInCollection = ccu.Name
		cl.ContentUnits = append(cl.ContentUnits, u)
	}

	return cl, nil
}

func handleContentUnits(db *sql.DB, r ContentUnitsRequest) (*ContentUnitsResponse, *HttpError) {
	mods := []qm.QueryMod{SECURE_PUBLISHED_MOD}

	// filters
	if err := appendIDsFilterMods(&mods, r.IDsFilter); err != nil {
		return nil, NewBadRequestError(err)
	}
	if err := appendContentTypesFilterMods(&mods, r.ContentTypesFilter); err != nil {
		return nil, NewBadRequestError(err)
	}
	if err := appendDateRangeFilterMods(&mods, r.DateRangeFilter); err != nil {
		return nil, NewBadRequestError(err)
	}
	if err := appendSourcesFilterMods(db, &mods, r.SourcesFilter); err != nil {
		if e, ok := err.(*HttpError); ok {
			return nil, e
		} else {
			return nil, NewInternalError(err)
		}
	}
	if err := appendTagsFilterMods(db, &mods, r.TagsFilter); err != nil {
		return nil, NewInternalError(err)
	}
	if err := appendGenresProgramsFilterMods(db, &mods, r.GenresProgramsFilter); err != nil {
		return nil, NewInternalError(err)
	}
	if err := appendCollectionsFilterMods(db, &mods, r.CollectionsFilter); err != nil {
		return nil, NewInternalError(err)
	}
	if err := appendPublishersFilterMods(db, &mods, r.PublishersFilter); err != nil {
		return nil, NewInternalError(err)
	}

	var total int64
	countMods := append([]qm.QueryMod{qm.Select("count(DISTINCT id)")}, mods...)
	err := mdbmodels.ContentUnits(db, countMods...).QueryRow().Scan(&total)
	if err != nil {
		return nil, NewInternalError(err)
	}
	if total == 0 {
		return NewContentUnitsResponse(), nil
	}

	// order, limit, offset
	_, offset, err := appendListMods(&mods, r.ListRequest)
	if err != nil {
		return nil, NewBadRequestError(err)
	}
	if int64(offset) >= total {
		return NewContentUnitsResponse(), nil
	}

	// Eager loading
	mods = append(mods, qm.Load(
		"CollectionsContentUnits",
		"CollectionsContentUnits.Collection"))

	// data query
	units, err := mdbmodels.ContentUnits(db, mods...).All()
	if err != nil {
		return nil, NewInternalError(err)
	}

	// response
	cus, ex := prepareCUs(db, units, r.Language)
	if ex != nil {
		return nil, ex
	}

	resp := &ContentUnitsResponse{
		ListResponse: ListResponse{Total: total},
		ContentUnits: cus,
	}

	return resp, nil
}

// units must be loaded with their CCUs loaded with their collections
func prepareCUs(db *sql.DB, units []*mdbmodels.ContentUnit, language string) ([]*ContentUnit, *HttpError) {

	// Filter secure published collections
	// Load i18n for all content units and all collections - total 2 DB round trips
	cuids := make([]int64, len(units))
	cids := make([]int64, 0)
	for i, x := range units {
		cuids[i] = x.ID
		b := x.R.CollectionsContentUnits[:0]
		for _, y := range x.R.CollectionsContentUnits {
			if consts.SEC_PUBLIC == y.R.Collection.Secure && y.R.Collection.Published {
				b = append(b, y)
				cids = append(cids, y.CollectionID)
			}
			x.R.CollectionsContentUnits = b
		}
	}

	cui18nsMap, err := loadCUI18ns(db, language, cuids)
	if err != nil {
		return nil, NewInternalError(err)
	}
	ci18nsMap, err := loadCI18ns(db, language, cids)
	if err != nil {
		return nil, NewInternalError(err)
	}

	cus := make([]*ContentUnit, len(units))
	for i, x := range units {
		cu, err := mdbToCU(x)
		if err != nil {
			return nil, NewInternalError(err)
		}
		if i18ns, ok := cui18nsMap[x.ID]; ok {
			setCUI18n(cu, language, i18ns)
		}

		// collections
		cu.Collections = make(map[string]*Collection, 0)
		for _, ccu := range x.R.CollectionsContentUnits {
			cl := ccu.R.Collection

			cc, err := mdbToC(cl)
			if err != nil {
				return nil, NewInternalError(err)
			}
			if i18ns, ok := ci18nsMap[cl.ID]; ok {
				setCI18n(cc, language, i18ns)
			}

			// Dirty hack for unique mapping - needs to parse in client...
			key := fmt.Sprintf("%s____%s", cl.UID, ccu.Name)
			cu.Collections[key] = cc
		}
		cus[i] = cu
	}

	return cus, nil
}

func handlePublishers(db *sql.DB, r PublishersRequest) (*PublishersResponse, *HttpError) {
	total, err := mdbmodels.Publishers(db).Count()
	if err != nil {
		return nil, NewInternalError(err)
	}
	if total == 0 {
		return NewPublishersResponse(), nil
	}

	// order, limit, offset
	mods := make([]qm.QueryMod, 0)
	r.OrderBy = "id"
	_, offset, err := appendListMods(&mods, r.ListRequest)
	if err != nil {
		return nil, NewBadRequestError(err)
	}
	if int64(offset) >= total {
		return NewPublishersResponse(), nil
	}

	// Eager loading
	mods = append(mods, qm.Load("PublisherI18ns"))

	// data query
	publishers, err := mdbmodels.Publishers(db, mods...).All()
	if err != nil {
		return nil, NewInternalError(err)
	}

	// response
	ps := make([]*Publisher, len(publishers))
	for i := range publishers {
		p := publishers[i]

		pp := &Publisher{
			UID: p.UID,
		}

		// i18ns
		for _, l := range consts.LANG_ORDER[r.Language] {
			for _, i18n := range p.R.PublisherI18ns {
				if i18n.Language == l {
					if !pp.Name.Valid && i18n.Name.Valid {
						pp.Name = i18n.Name
					}
					if !pp.Description.Valid && i18n.Description.Valid {
						pp.Description = i18n.Description
					}
				}
			}
		}

		ps[i] = pp
	}

	resp := &PublishersResponse{
		ListResponse: ListResponse{Total: total},
		Publishers:   ps,
	}

	return resp, nil
}

func handleRecentlyUpdated(db *sql.DB) ([]CollectionUpdateStatus, *HttpError) {
	q := `SELECT
  c.uid,
  max(cu.properties ->> 'film_date') max_film_date,
  count(cu.id)
FROM collections c INNER JOIN collections_content_units ccu
    ON c.id = ccu.collection_id AND c.type_id = 5 AND c.secure = 0 AND c.published IS TRUE
  INNER JOIN content_units cu
    ON ccu.content_unit_id = cu.id AND cu.secure = 0 AND cu.published IS TRUE AND cu.properties ? 'film_date'
GROUP BY c.id
ORDER BY max_film_date DESC`

	rows, err := queries.Raw(db, q).Query()
	if err != nil {
		return nil, NewInternalError(err)
	}
	defer rows.Close()

	data := make([]CollectionUpdateStatus, 0)
	for rows.Next() {
		var x CollectionUpdateStatus
		err := rows.Scan(&x.UID, &x.LastUpdate, &x.UnitsCount)
		if err != nil {
			return nil, NewInternalError(err)
		}
		data = append(data, x)
	}
	if err := rows.Err(); err != nil {
		return nil, NewInternalError(err)
	}

	return data, nil
}

// appendListMods compute and appends the OrderBy, Limit and Offset query mods.
// It returns the limit, offset and error if any
func appendListMods(mods *[]qm.QueryMod, r ListRequest) (int, int, error) {

	// group by id to remove duplicates
	*mods = append(*mods, qm.GroupBy("id"))

	if r.OrderBy == "" {
		*mods = append(*mods,
			qm.OrderBy("(coalesce(properties->>'film_date', properties->>'start_date', created_at::text))::date desc, created_at desc"))
	} else {
		*mods = append(*mods, qm.OrderBy(r.OrderBy))
	}

	var limit, offset int

	if r.StartIndex == 0 {
		// pagination style
		if r.PageSize == 0 {
			limit = consts.API_DEFAULT_PAGE_SIZE
		} else {
			limit = utils.Min(r.PageSize, consts.API_MAX_PAGE_SIZE)
		}
		if r.PageNumber > 1 {
			offset = (r.PageNumber - 1) * limit
		}
	} else {
		// start & stop index style for "infinite" lists
		offset = r.StartIndex - 1
		if r.StopIndex == 0 {
			limit = consts.API_MAX_PAGE_SIZE
		} else if r.StopIndex < r.StartIndex {
			return 0, 0, errors.Errorf("Invalid range [%d-%d]", r.StartIndex, r.StopIndex)
		} else {
			limit = r.StopIndex - r.StartIndex + 1
		}
	}

	*mods = append(*mods, qm.Limit(limit))
	if offset != 0 {
		*mods = append(*mods, qm.Offset(offset))
	}

	return limit, offset, nil
}

func appendIDsFilterMods(mods *[]qm.QueryMod, f IDsFilter) error {
	if utils.IsEmpty(f.IDs) {
		return nil
	}

	*mods = append(*mods, qm.WhereIn("uid IN ?", utils.ConvertArgsString(f.IDs)...))

	return nil
}

func appendContentTypesFilterMods(mods *[]qm.QueryMod, f ContentTypesFilter) error {
	if utils.IsEmpty(f.ContentTypes) {
		return nil
	}

	a := make([]interface{}, len(f.ContentTypes))
	for i, x := range f.ContentTypes {
		ct, ok := mdb.CONTENT_TYPE_REGISTRY.ByName[strings.ToUpper(x)]
		if ok {
			a[i] = ct.ID
		} else {
			return errors.Errorf("Unknown content type: %s", x)
		}
	}

	*mods = append(*mods, qm.WhereIn("type_id IN ?", a...))

	return nil
}

func appendDateRangeFilterMods(mods *[]qm.QueryMod, f DateRangeFilter) error {
	s, e, err := f.Range()
	if err != nil {
		return err
	}

	if f.StartDate != "" && f.EndDate != "" && e.Before(s) {
		return errors.New("Invalid date range")
	}

	if f.StartDate != "" {
		*mods = append(*mods, qm.Where("(properties->>'film_date')::date >= ?", s))
	}
	if f.EndDate != "" {
		*mods = append(*mods, qm.Where("(properties->>'film_date')::date <= ?", e))
	}

	return nil
}

func appendSourcesFilterMods(exec boil.Executor, mods *[]qm.QueryMod, f SourcesFilter) error {
	if utils.IsEmpty(f.Authors) && len(f.Sources) == 0 {
		return nil
	}

	// slice of all source ids we want
	source_uids := make([]string, 0)

	// fetch source ids by authors
	if !utils.IsEmpty(f.Authors) {
		for _, x := range f.Authors {
			if _, ok := mdb.AUTHOR_REGISTRY.ByCode[strings.ToLower(x)]; !ok {
				return NewBadRequestError(errors.Errorf("Unknown author: %s", x))
			}
		}

		var uids pq.StringArray
		q := `SELECT array_agg(DISTINCT s.uid)
		      FROM authors a INNER JOIN authors_sources "as" ON a.id = "as".author_id
		      INNER JOIN sources s ON "as".source_id = s.id
		      WHERE a.code = ANY($1)`
		err := queries.Raw(exec, q, pq.Array(f.Authors)).QueryRow().Scan(&uids)
		if err != nil {
			return err
		}
		source_uids = append(source_uids, uids...)
	}

	// blend in requested sources
	source_uids = append(source_uids, f.Sources...)

	// find all nested source_uids
	q := `WITH RECURSIVE rec_sources AS (
		  SELECT s.id FROM sources s WHERE s.uid = ANY($1)
		  UNION
		  SELECT s.id FROM sources s INNER JOIN rec_sources rs ON s.parent_id = rs.id
	      )
	      SELECT array_agg(distinct id) FROM rec_sources`
	var ids pq.Int64Array
	err := queries.Raw(exec, q, pq.Array(source_uids)).QueryRow().Scan(&ids)
	if err != nil {
		return err
	}

	if ids == nil || len(ids) == 0 {
		*mods = append(*mods, qm.Where("id < 0")) // so results would be empty
	} else {
		*mods = append(*mods,
			qm.InnerJoin("content_units_sources cus ON id = cus.content_unit_id"),
			qm.WhereIn("cus.source_id in ?", utils.ConvertArgsInt64(ids)...))
	}

	return nil
}

func appendTagsFilterMods(exec boil.Executor, mods *[]qm.QueryMod, f TagsFilter) error {
	if len(f.Tags) == 0 {
		return nil
	}

	// find all nested tag_ids
	q := `WITH RECURSIVE rec_tags AS (
	        SELECT t.id FROM tags t WHERE t.uid = ANY($1)
	        UNION
	        SELECT t.id FROM tags t INNER JOIN rec_tags rt ON t.parent_id = rt.id
	      )
	      SELECT array_agg(distinct id) FROM rec_tags`
	var ids pq.Int64Array
	err := queries.Raw(exec, q, pq.Array(f.Tags)).QueryRow().Scan(&ids)
	if err != nil {
		return err
	}

	if ids == nil || len(ids) == 0 {
		*mods = append(*mods, qm.Where("id < 0")) // so results would be empty
	} else {
		*mods = append(*mods,
			qm.InnerJoin("content_units_tags cut ON id = cut.content_unit_id"),
			qm.WhereIn("cut.tag_id in ?", utils.ConvertArgsInt64(ids)...))
	}

	return nil
}

func appendGenresProgramsFilterMods(exec boil.Executor, mods *[]qm.QueryMod, f GenresProgramsFilter) error {
	if len(f.Genres) == 0 && len(f.Programs) == 0 {
		return nil
	}

	var ids pq.Int64Array
	if len(f.Programs) > 0 {
		// convert collections uids to ids
		q := `SELECT array_agg(DISTINCT id) FROM collections WHERE uid = ANY($1)`
		err := queries.Raw(exec, q, pq.Array(f.Programs)).QueryRow().Scan(&ids)
		if err != nil {
			return err
		}
	} else {
		// find collections by genres
		q := `SELECT array_agg(DISTINCT id) FROM collections WHERE type_id = $1 AND properties -> 'genres' ?| $2`
		err := queries.Raw(exec, q,
			mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_VIDEO_PROGRAM].ID,
			pq.Array(f.Genres)).
			QueryRow().Scan(&ids)
		if err != nil {
			return err
		}
	}

	if ids == nil || len(ids) == 0 {
		*mods = append(*mods, qm.Where("id < 0")) // so results would be empty
	} else {
		*mods = append(*mods,
			qm.InnerJoin("collections_content_units ccu ON id = ccu.content_unit_id"),
			qm.WhereIn("ccu.collection_id in ?", utils.ConvertArgsInt64(ids)...))
	}

	return nil
}

func appendCollectionsFilterMods(exec boil.Executor, mods *[]qm.QueryMod, f CollectionsFilter) error {
	if len(f.Collections) == 0 {
		return nil
	}

	// convert collections uids to ids
	var ids pq.Int64Array
	q := `SELECT array_agg(DISTINCT id) FROM collections WHERE uid = ANY($1) AND secure = 0 AND published IS TRUE`
	err := queries.Raw(exec, q, pq.Array(f.Collections)).QueryRow().Scan(&ids)
	if err != nil {
		return err
	}

	if ids == nil || len(ids) == 0 {
		*mods = append(*mods, qm.Where("id < 0")) // so results would be empty
	} else {
		*mods = append(*mods,
			qm.InnerJoin("collections_content_units ccu ON id = ccu.content_unit_id"),
			qm.WhereIn("ccu.collection_id in ?", utils.ConvertArgsInt64(ids)...))
	}

	return nil
}

func appendPublishersFilterMods(exec boil.Executor, mods *[]qm.QueryMod, f PublishersFilter) error {
	if len(f.Publishers) == 0 {
		return nil
	}

	// convert publisher uids to ids
	var ids pq.Int64Array
	q := `SELECT array_agg(DISTINCT id) FROM publishers WHERE uid = ANY($1)`
	err := queries.Raw(exec, q, pq.Array(f.Publishers)).QueryRow().Scan(&ids)
	if err != nil {
		return err
	}

	if ids == nil || len(ids) == 0 {
		*mods = append(*mods, qm.Where("id < 0")) // so results would be empty
	} else {
		q := `content_unit_derivations cud ON id = cud.source_id AND cud.derived_id IN
(SELECT cu.id FROM content_units cu
INNER JOIN content_units_publishers cup ON cu.id = cup.content_unit_id
AND cu.secure = 0 AND cu.published IS TRUE AND cup.publisher_id = ANY(?))`
		*mods = append(*mods, qm.InnerJoin(q, ids))
	}

	return nil
}

// concludeRequest responds with JSON of given response or aborts the request with the given error.
func concludeRequest(c *gin.Context, resp interface{}, err *HttpError) {
	if err == nil {
		c.JSON(http.StatusOK, resp)
	} else {
		err.Abort(c)
	}
}

func mdbToC(c *mdbmodels.Collection) (cl *Collection, err error) {
	var props mdb.CollectionProperties
	if err = c.Properties.Unmarshal(&props); err != nil {
		err = errors.Wrap(err, "json.Unmarshal properties")
		return
	}

	cl = &Collection{
		ID:              c.UID,
		ContentType:     mdb.CONTENT_TYPE_REGISTRY.ByID[c.TypeID].Name,
		Country:         props.Country,
		City:            props.City,
		FullAddress:     props.FullAddress,
		Genres:          props.Genres,
		DefaultLanguage: props.DefaultLanguage,
		HolidayID:       props.HolidayTag,
	}

	if !props.FilmDate.IsZero() {
		cl.FilmDate = &utils.Date{Time: props.FilmDate.Time}
	}
	if !props.StartDate.IsZero() {
		cl.StartDate = &utils.Date{Time: props.StartDate.Time}
	}
	if !props.EndDate.IsZero() {
		cl.EndDate = &utils.Date{Time: props.EndDate.Time}
	}

	return
}

func mdbToCU(cu *mdbmodels.ContentUnit) (*ContentUnit, error) {
	var props mdb.ContentUnitProperties
	if err := cu.Properties.Unmarshal(&props); err != nil {
		return nil, errors.Wrap(err, "json.Unmarshal properties")
	}

	u := &ContentUnit{
		ID:               cu.UID,
		ContentType:      mdb.CONTENT_TYPE_REGISTRY.ByID[cu.TypeID].Name,
		Duration:         props.Duration,
		OriginalLanguage: props.OriginalLanguage,
	}

	if !props.FilmDate.IsZero() {
		u.FilmDate = &utils.Date{Time: props.FilmDate.Time}
	}

	return u, nil
}

func mdbToFile(file *mdbmodels.File) (*File, error) {
	var props mdb.FileProperties
	if err := file.Properties.Unmarshal(&props); err != nil {
		return nil, errors.Wrap(err, "json.Unmarshal properties")
	}

	f := &File{
		ID:       file.UID,
		Name:     file.Name,
		Size:     file.Size,
		Type:     file.Type,
		SubType:  file.SubType,
		Duration: props.Duration,
	}

	if file.Language.Valid {
		f.Language = file.Language.String
	}
	if file.MimeType.Valid {
		f.MimeType = file.MimeType.String
	}

	return f, nil
}

func loadCI18ns(db *sql.DB, language string, ids []int64) (map[int64]map[string]*mdbmodels.CollectionI18n, error) {
	if len(ids) == 0 {
		return make(map[int64]map[string]*mdbmodels.CollectionI18n, 0), nil
	}

	// Load from DB
	i18ns, err := mdbmodels.CollectionI18ns(db,
		qm.WhereIn("collection_id in ?", utils.ConvertArgsInt64(ids)...),
		qm.AndIn("language in ?", utils.ConvertArgsString(consts.LANG_ORDER[language])...)).
		All()
	if err != nil {
		return nil, errors.Wrap(err, "Load collections i18ns from DB")
	}

	// Group by collection and language
	i18nsMap := make(map[int64]map[string]*mdbmodels.CollectionI18n, len(ids))
	for _, x := range i18ns {
		v, ok := i18nsMap[x.CollectionID]
		if !ok {
			v = make(map[string]*mdbmodels.CollectionI18n, 1)
			i18nsMap[x.CollectionID] = v
		}
		v[x.Language] = x
	}

	return i18nsMap, nil
}

func setCI18n(c *Collection, language string, i18ns map[string]*mdbmodels.CollectionI18n) {
	for _, l := range consts.LANG_ORDER[language] {
		li18n, ok := i18ns[l]
		if ok {
			if c.Name == "" && li18n.Name.Valid {
				c.Name = li18n.Name.String
			}
			if c.Description == "" && li18n.Description.Valid {
				c.Description = li18n.Description.String
			}
		}
	}
}

func loadCUI18ns(db *sql.DB, language string, ids []int64) (map[int64]map[string]*mdbmodels.ContentUnitI18n, error) {
	// Load from DB
	i18ns, err := mdbmodels.ContentUnitI18ns(db,
		qm.WhereIn("content_unit_id in ?", utils.ConvertArgsInt64(ids)...),
		qm.AndIn("language in ?", utils.ConvertArgsString(consts.LANG_ORDER[language])...)).
		All()
	if err != nil {
		return nil, errors.Wrap(err, "Load content units i18ns from DB")
	}

	// Group by content unit and language
	i18nsMap := make(map[int64]map[string]*mdbmodels.ContentUnitI18n, len(ids))
	for _, x := range i18ns {
		v, ok := i18nsMap[x.ContentUnitID]
		if !ok {
			v = make(map[string]*mdbmodels.ContentUnitI18n, 1)
			i18nsMap[x.ContentUnitID] = v
		}
		v[x.Language] = x
	}

	return i18nsMap, nil
}

func loadCUFiles(db *sql.DB, ids []int64) (map[int64][]*mdbmodels.File, error) {
	// Load from DB
	allFiles, err := mdbmodels.Files(db,
		SECURE_PUBLISHED_MOD,
		qm.WhereIn("content_unit_id in ? and removed_at is null", utils.ConvertArgsInt64(ids)...)).
		All()
	if err != nil {
		return nil, errors.Wrap(err, "Load files from DB")
	}

	// Group by content unit
	filesMap := make(map[int64][]*mdbmodels.File, len(ids))
	for _, x := range allFiles {
		v, ok := filesMap[x.ContentUnitID.Int64]
		if ok {
			v = append(v, x)
		} else {
			v = []*mdbmodels.File{x}
		}
		filesMap[x.ContentUnitID.Int64] = v
	}

	return filesMap, nil
}

func setCUI18n(cu *ContentUnit, language string, i18ns map[string]*mdbmodels.ContentUnitI18n) {
	for _, l := range consts.LANG_ORDER[language] {
		li18n, ok := i18ns[l]
		if ok {
			if cu.Name == "" && li18n.Name.Valid {
				cu.Name = li18n.Name.String
			}
			if cu.Description == "" && li18n.Description.Valid {
				cu.Description = li18n.Description.String
			}
		}
	}
}

func setCUFiles(cu *ContentUnit, files []*mdbmodels.File) error {
	cu.Files = make([]*File, len(files))

	for i, x := range files {
		f, err := mdbToFile(x)
		if err != nil {
			return err
		}
		cu.Files[i] = f
	}

	return nil
}
