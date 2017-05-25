package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
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

	db := c.MustGet("MDB_DB").(*sql.DB)

	mods := []qm.QueryMod{SECURE_PUBLISHED_MOD}

	// filters
	if err := appendContentTypesFilterMods(&mods, r.ContentTypesFilter); err != nil {
		NewBadRequestError(err).Abort(c)
		return
	}
	if err := appendDateRangeFilterMods(&mods, r.DateRangeFilter); err != nil {
		NewBadRequestError(err).Abort(c)
		return
	}

	// count query
	total, err := mdbmodels.Collections(db, mods...).Count()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	if total == 0 {
		c.JSON(http.StatusOK, NewCollectionsResponse())
		return
	}

	// order, limit, offset
	if err = appendListMods(&mods, r.ListRequest); err != nil {
		NewBadRequestError(err).Abort(c)
		return
	}

	// Eager loading
	mods = append(mods, qm.Load(
		"CollectionsContentUnits",
		"CollectionsContentUnits.ContentUnit"))

	// data query
	collections, err := mdbmodels.Collections(db, mods...).All()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	// Filter secure published content units
	// Load i18n for all collections and all units - total 2 DB round trips
	cids := make([]interface{}, len(collections))
	cuids := make([]interface{}, 0)
	for i, x := range collections {
		cids[i] = x.ID
		b := x.R.CollectionsContentUnits[:0]
		for _, y := range x.R.CollectionsContentUnits {
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
		NewInternalError(err).Abort(c)
		return
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
		NewInternalError(err).Abort(c)
		return
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
	resp := CollectionsResponse{
		ListResponse: ListResponse{Total: total},
		Collections:  make([]*Collection, len(collections)),
	}
	for i, x := range collections {
		var props mdb.CollectionProperties
		err = x.Properties.Unmarshal(&props)
		if err != nil {
			NewInternalError(err).Abort(c)
			return
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
				NewInternalError(err).Abort(c)
				return
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

	c.JSON(http.StatusOK, resp)
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

func appendListMods(mods *[]qm.QueryMod, r ListRequest) error {
	if r.OrderBy == "" {
		*mods = append(*mods, qm.OrderBy("id"))
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
			return errors.Errorf("Invalid range [%d-%d]", r.StartIndex, r.StopIndex)
		} else {
			limit = r.StopIndex - r.StartIndex + 1
		}
	}

	*mods = append(*mods, qm.Limit(limit))
	if offset != 0 {
		*mods = append(*mods, qm.Offset(offset))
	}

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
