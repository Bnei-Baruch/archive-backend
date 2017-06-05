package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/vattle/sqlboiler/boil"
	"github.com/vattle/sqlboiler/queries"
	"github.com/vattle/sqlboiler/queries/qm"
	"gopkg.in/gin-gonic/gin.v1"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var SECURE_PUBLISHED_MOD = qm.Where(fmt.Sprintf("secure=%d AND published IS TRUE", mdb.SEC_PUBLIC))

func CollectionsHandler(c *gin.Context) {
	var r CollectionsRequest
	if c.Bind(&r) != nil {
		return
	}

	resp, err := handleCollections(c.MustGet("MDB_DB").(*sql.DB), r)
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
		qm.Where("uid = ?", uid)).
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

	var props mdb.ContentUnitProperties
	err = cu.Properties.Unmarshal(&props)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	u := &ContentUnit{
		ID:               cu.UID,
		ContentType:      mdb.CONTENT_TYPE_REGISTRY.ByID[cu.TypeID].Name,
		FilmDate:         Date{Time: props.FilmDate.Time},
		Duration:         props.Duration,
		OriginalLanguage: props.OriginalLanguage,
	}

	// i18n
	cui18ns, err := mdbmodels.ContentUnitI18ns(db,
		qm.Where("content_unit_id = ?", cu.ID),
		qm.AndIn("language in ?", utils.ConvertArgsString(LANG_ORDER[r.Language])...)).
		All()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	for _, l := range LANG_ORDER[r.Language] {
		for _, i18n := range cui18ns {
			if i18n.Language == l {
				if u.Name == "" && i18n.Name.Valid {
					u.Name = i18n.Name.String
				}
				if u.Description == "" && i18n.Description.Valid {
					u.Description = i18n.Description.String
				}
			}
		}
	}

	// files
	files, err := mdbmodels.Files(db,
		SECURE_PUBLISHED_MOD,
		qm.Where("content_unit_id = ?", cu.ID)).
		All()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	u.Files = make([]*File, len(files))
	for i, x := range files {
		var props mdb.FileProperties
		err := x.Properties.Unmarshal(&props)
		if err != nil {
			NewInternalError(err).Abort(c)
			return
		}

		f := &File{
			ID:          x.UID,
			Name:        x.Name,
			Size:        x.Size,
			Type:        x.Type,
			SubType:     x.SubType,
			URL:         props.URL,
			DownloadURL: props.URL,
			Duration:    props.Duration,
		}

		if x.Language.Valid {
			f.Language = x.Language.String
		}
		if x.MimeType.Valid {
			f.MimeType = x.MimeType.String
		}

		u.Files[i] = f
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
		cr := CollectionsRequest{
			ContentTypesFilter: ContentTypesFilter{
				ContentTypes: []string{mdb.CT_DAILY_LESSON, mdb.CT_SATURDAY_LESSON},
			},
			ListRequest:     r.ListRequest,
			DateRangeFilter: r.DateRangeFilter,
		}
		resp, err := handleCollections(c.MustGet("MDB_DB").(*sql.DB), cr)
		concludeRequest(c, resp, err)
	} else {
		cur := ContentUnitsRequest{
			ContentTypesFilter: ContentTypesFilter{
				ContentTypes: []string{mdb.CT_LESSON_PART},
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

func SearchHandler(c *gin.Context) {
	text := c.Query("text")
	if text == "" {
		NewBadRequestError(errors.New("Can't search for an empty text")).Abort(c)
		return
	}

	page := 0
	pageQ := c.Query("page")
	if pageQ != "" {
		var err error
		page, err = strconv.Atoi(pageQ)
		if err != nil {
			NewBadRequestError(err).Abort(c)
			return
		}
	}

	res, err := handleSearch(c.MustGet("ES_CLIENT").(*elastic.Client), "mdb_collections", text, page)
	if err == nil {
		c.JSON(http.StatusOK, res)
	} else {
		NewInternalError(err).Abort(c)
	}
}

func handleCollections(db *sql.DB, r CollectionsRequest) (*CollectionsResponse, *HttpError) {
	mods := []qm.QueryMod{SECURE_PUBLISHED_MOD}

	// filters
	if err := appendContentTypesFilterMods(&mods, r.ContentTypesFilter); err != nil {
		return nil, NewBadRequestError(err)
	}
	if err := appendDateRangeFilterMods(&mods, r.DateRangeFilter); err != nil {
		return nil, NewBadRequestError(err)
	}

	// count query
	total, err := mdbmodels.Collections(db, mods...).Count()
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

	// Eager loading
	mods = append(mods, qm.Load(
		"CollectionsContentUnits",
		"CollectionsContentUnits.ContentUnit"))

	// data query
	collections, err := mdbmodels.Collections(db, mods...).All()
	if err != nil {
		return nil, NewInternalError(err)
	}

	// Filter secure & published content units
	// Load i18n for all collections and all units - total 2 DB round trips
	cids := make([]interface{}, len(collections))
	cuids := make([]interface{}, 0)
	for i, x := range collections {
		cids[i] = x.ID
		b := x.R.CollectionsContentUnits[:0]
		for _, y := range x.R.CollectionsContentUnits {

			// Workaround for this bug: https://github.com/vattle/sqlboiler/issues/154
			if y.R.ContentUnit == nil {
				err = y.L.LoadContentUnit(db, true, y)
				if err != nil {
					return nil, NewInternalError(err)
				}
			}

			if mdb.SEC_PUBLIC == y.R.ContentUnit.Secure && y.R.ContentUnit.Published {
				b = append(b, y)
				cuids = append(cuids, y.ContentUnitID)
			}
			x.R.CollectionsContentUnits = b
		}
	}
	ci18ns, err := mdbmodels.CollectionI18ns(db,
		qm.WhereIn("collection_id in ?", cids...),
		qm.AndIn("language in ?", utils.ConvertArgsString(LANG_ORDER[r.Language])...)).
		All()
	if err != nil {
		return nil, NewInternalError(err)
	}
	ci18nsMap := make(map[int64]map[string]*mdbmodels.CollectionI18n, len(cids))
	for _, x := range ci18ns {
		v, ok := ci18nsMap[x.CollectionID]
		if !ok {
			v = make(map[string]*mdbmodels.CollectionI18n, 1)
			ci18nsMap[x.CollectionID] = v
		}
		v[x.Language] = x
	}

	cui18ns, err := mdbmodels.ContentUnitI18ns(db,
		qm.WhereIn("content_unit_id in ?", cuids...),
		qm.AndIn("language in ?", utils.ConvertArgsString(LANG_ORDER[r.Language])...)).
		All()
	if err != nil {
		return nil, NewInternalError(err)
	}
	cui18nsMap := make(map[int64]map[string]*mdbmodels.ContentUnitI18n, len(cuids))
	for _, x := range cui18ns {
		v, ok := cui18nsMap[x.ContentUnitID]
		if !ok {
			v = make(map[string]*mdbmodels.ContentUnitI18n, 1)
			cui18nsMap[x.ContentUnitID] = v
		}
		v[x.Language] = x
	}

	// Response
	resp := &CollectionsResponse{
		ListResponse: ListResponse{Total: total},
		Collections:  make([]*Collection, len(collections)),
	}
	for i, x := range collections {
		var props mdb.CollectionProperties
		err = x.Properties.Unmarshal(&props)
		if err != nil {
			return nil, NewInternalError(err)
		}
		cl := &Collection{
			ID:          x.UID,
			ContentType: mdb.CONTENT_TYPE_REGISTRY.ByID[x.TypeID].Name,
			FilmDate:    Date{Time: props.FilmDate.Time},
		}

		// i18n - get from map by lang order
		i18ns, ok := ci18nsMap[x.ID]
		if ok {
			for _, l := range LANG_ORDER[r.Language] {
				li18n, ok := i18ns[l]
				if ok {
					if cl.Name == "" && li18n.Name.Valid {
						cl.Name = li18n.Name.String
					}
					if cl.Description == "" && li18n.Description.Valid {
						cl.Description = li18n.Description.String
					}
				}
			}
		}

		// content units
		sort.Sort(mdb.InCollection{ExtCCUSlice: mdb.ExtCCUSlice(x.R.CollectionsContentUnits)})
		cl.ContentUnits = make([]*ContentUnit, 0)
		for _, ccu := range x.R.CollectionsContentUnits {
			cu := ccu.R.ContentUnit
			var props mdb.ContentUnitProperties
			err = cu.Properties.Unmarshal(&props)
			if err != nil {
				return nil, NewInternalError(err)
			}
			u := &ContentUnit{
				ID:               cu.UID,
				ContentType:      mdb.CONTENT_TYPE_REGISTRY.ByID[cu.TypeID].Name,
				NameInCollection: ccu.Name,
				FilmDate:         Date{Time: props.FilmDate.Time},
				Duration:         props.Duration,
				OriginalLanguage: props.OriginalLanguage,
			}

			// i18n - get from map by lang order
			i18ns, ok := cui18nsMap[cu.ID]
			if ok {
				for _, l := range LANG_ORDER[r.Language] {
					li18n, ok := i18ns[l]
					if ok {
						if u.Name == "" && li18n.Name.Valid {
							u.Name = li18n.Name.String
						}
						if u.Description == "" && li18n.Description.Valid {
							u.Description = li18n.Description.String
						}
					}
				}
			}

			cl.ContentUnits = append(cl.ContentUnits, u)
		}
		resp.Collections[i] = cl
	}

	return resp, nil
}

func handleContentUnits(db *sql.DB, r ContentUnitsRequest) (*ContentUnitsResponse, *HttpError) {
	mods := []qm.QueryMod{SECURE_PUBLISHED_MOD}

	// filters
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

	// count query
	total, err := mdbmodels.ContentUnits(db, mods...).Count()
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

	// Filter secure published collections
	// Load i18n for all content units and all collections - total 2 DB round trips
	cuids := make([]interface{}, len(units))
	cids := make([]interface{}, 0)
	for i, x := range units {
		cuids[i] = x.ID
		b := x.R.CollectionsContentUnits[:0]
		for _, y := range x.R.CollectionsContentUnits {

			// Workaround for this bug: https://github.com/vattle/sqlboiler/issues/154
			if y.R.Collection == nil {
				err = y.L.LoadCollection(db, true, y)
				if err != nil {
					return nil, NewInternalError(err)
				}
			}

			if mdb.SEC_PUBLIC == y.R.Collection.Secure && y.R.Collection.Published {
				b = append(b, y)
				cids = append(cids, y.CollectionID)
			}
			x.R.CollectionsContentUnits = b
		}
	}

	cui18ns, err := mdbmodels.ContentUnitI18ns(db,
		qm.WhereIn("content_unit_id in ?", cuids...),
		qm.AndIn("language in ?", utils.ConvertArgsString(LANG_ORDER[r.Language])...)).
		All()
	if err != nil {
		return nil, NewInternalError(err)
	}
	cui18nsMap := make(map[int64]map[string]*mdbmodels.ContentUnitI18n, len(cuids))
	for _, x := range cui18ns {
		v, ok := cui18nsMap[x.ContentUnitID]
		if !ok {
			v = make(map[string]*mdbmodels.ContentUnitI18n, 1)
			cui18nsMap[x.ContentUnitID] = v
		}
		v[x.Language] = x
	}

	ci18ns, err := mdbmodels.CollectionI18ns(db,
		qm.WhereIn("collection_id in ?", cids...),
		qm.AndIn("language in ?", utils.ConvertArgsString(LANG_ORDER[r.Language])...)).
		All()
	if err != nil {
		return nil, NewInternalError(err)
	}
	ci18nsMap := make(map[int64]map[string]*mdbmodels.CollectionI18n, len(cids))
	for _, x := range ci18ns {
		v, ok := ci18nsMap[x.CollectionID]
		if !ok {
			v = make(map[string]*mdbmodels.CollectionI18n, 1)
			ci18nsMap[x.CollectionID] = v
		}
		v[x.Language] = x
	}

	// Response
	resp := &ContentUnitsResponse{
		ListResponse: ListResponse{Total: total},
		ContentUnits: make([]*ContentUnit, len(units)),
	}
	for i, x := range units {
		var props mdb.ContentUnitProperties
		err = x.Properties.Unmarshal(&props)
		if err != nil {
			return nil, NewInternalError(err)

		}
		cu := &ContentUnit{
			ID:               x.UID,
			ContentType:      mdb.CONTENT_TYPE_REGISTRY.ByID[x.TypeID].Name,
			FilmDate:         Date{Time: props.FilmDate.Time},
			Duration:         props.Duration,
			OriginalLanguage: props.OriginalLanguage,
		}

		// i18n - get from map by lang order
		i18ns, ok := cui18nsMap[x.ID]
		if ok {
			for _, l := range LANG_ORDER[r.Language] {
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

		// collections
		cu.Collections = make(map[string]*Collection, 0)
		for _, ccu := range x.R.CollectionsContentUnits {
			cl := ccu.R.Collection
			var props mdb.CollectionProperties
			err = cl.Properties.Unmarshal(&props)
			if err != nil {
				return nil, NewInternalError(err)
			}
			cc := &Collection{
				ID:          cl.UID,
				ContentType: mdb.CONTENT_TYPE_REGISTRY.ByID[cl.TypeID].Name,
				FilmDate:    Date{Time: props.FilmDate.Time},
			}

			// i18n - get from map by lang order
			i18ns, ok := ci18nsMap[cl.ID]
			if ok {
				for _, l := range LANG_ORDER[r.Language] {
					li18n, ok := i18ns[l]
					if ok {
						if cc.Name == "" && li18n.Name.Valid {
							cc.Name = li18n.Name.String
						}
						if cc.Description == "" && li18n.Description.Valid {
							cc.Description = li18n.Description.String
						}
					}
				}
			}

			// Dirty hack for unique mapping - needs to parse in client...
			key := fmt.Sprintf("%s____%s", cl.UID, ccu.Name)
			cu.Collections[key] = cc
		}
		resp.ContentUnits[i] = cu
	}

	return resp, nil
}

func handleSearch(esc *elastic.Client, index string, text string, from int) (*elastic.SearchResult, error) {
	q := elastic.NewNestedQuery("content_units",
		elastic.NewMultiMatchQuery(text, "content_units.names.*", "content_units.descriptions.*"))

	h := elastic.NewHighlight().HighlighQuery(q)

	return esc.Search().
		Index(index).
		Query(q).
		Highlight(h).
		From(from).
		Do(context.TODO())
}

// appendListMods compute and appends the OrderBy, Limit and Offset query mods.
// It returns the limit, offset and error if any
func appendListMods(mods *[]qm.QueryMod, r ListRequest) (int, int, error) {
	if r.OrderBy == "" {
		*mods = append(*mods, qm.OrderBy("id desc"))
	} else {
		*mods = append(*mods, qm.OrderBy(r.OrderBy))
	}

	var limit, offset int

	if r.StartIndex == 0 {
		// pagination style
		if r.PageSize == 0 {
			limit = DEFAULT_PAGE_SIZE
		} else {
			limit = utils.Min(r.PageSize, MAX_PAGE_SIZE)
		}
		if r.PageNumber > 1 {
			offset = (r.PageNumber - 1) * limit
		}
	} else {
		// start & stop index style for "infinite" lists
		offset = r.StartIndex - 1
		if r.StopIndex == 0 {
			limit = MAX_PAGE_SIZE
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

	if len(source_uids) == 0 {
		return nil
	}

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

// concludeRequest responds with JSON of given response or aborts the request with the given error.
func concludeRequest(c *gin.Context, resp interface{}, err *HttpError) {
	if err == nil {
		c.JSON(http.StatusOK, resp)
	} else {
		err.Abort(c)
	}
}
