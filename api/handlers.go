package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/vattle/sqlboiler/queries/qm"
	"gopkg.in/gin-gonic/gin.v1"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

func CollectionsHandler(c *gin.Context) {
	var r CollectionsRequest
	if c.Bind(&r) != nil {
		return
	}
	db := c.MustGet("MDB_DB").(*sql.DB)

	mods := make([]qm.QueryMod, 0)

	if !utils.IsEmpty(r.ContentType) {
		a := make([]interface{}, len(r.ContentType))
		for i, x := range r.ContentType {
			ct, ok := mdb.CONTENT_TYPE_REGISTRY.ByName[strings.ToUpper(x)]
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{
					"status": "error",
					"error":  fmt.Sprintf("Unknown content type: %s", x)})
				return
			}
			a[i] = ct.ID
		}
		mods = append(mods, qm.WhereIn("type_id in ?", a...))
	}

	if r.StartDate != nil {
		mods = append(mods, qm.Where("(properties->>'film_date')::date >= ?", r.StartDate.Time))
	}
	if r.EndDate != nil {
		mods = append(mods, qm.Where("(properties->>'film_date')::date <= ?", r.EndDate.Time))
	}

	total, err := mdbmodels.Collections(db, mods...).Count()
	if err != nil {
		internalServerError(c, err)
		return
	}
	if total == 0 {
		c.JSON(http.StatusOK, NewCollectionsResponse())
		return
	}

	if r.OrderBy == "" {
		mods = append(mods, qm.OrderBy("id"))
	} else {
		mods = append(mods, qm.OrderBy(r.OrderBy))
	}

	var pageSize int
	if r.PageSize == 0 {
		pageSize = DEFAULT_PAGE_SIZE
	} else {
		pageSize = utils.Min(r.PageSize, MAX_PAGE_SIZE)
	}
	mods = append(mods, qm.Limit(pageSize))
	if r.PageNumber > 1 {
		offset := (r.PageNumber - 1)  * pageSize
		if total < int64(offset) {
			c.JSON(http.StatusOK, NewCollectionsResponse())
			return
		}
		mods = append(mods, qm.Offset(offset))
	}

	mods = append(mods, qm.Load(
		"CollectionsContentUnits",
		"CollectionsContentUnits.ContentUnit"))

	collections, err := mdbmodels.Collections(db, mods...).All()
	if err != nil {
		internalServerError(c, err)
		return
	}

	// Load i18n for all collections and all units - total 2 DB round trips
	cids := make([]interface{}, len(collections))
	cuids := make([]interface{}, 0)
	for i, x := range collections {
		cids[i] = x.ID
		for _, y := range x.R.CollectionsContentUnits {
			cuids = append(cuids, y.ContentUnitID)
		}
	}
	ci18ns, err := mdbmodels.CollectionI18ns(db,
		qm.WhereIn("collection_id in ?", cids...),
		qm.AndIn("language in ?", utils.ConvertArgsString(LANG_ORDER[r.Language])...)).
		All()
	if err != nil {
		internalServerError(c, err)
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
		internalServerError(c, err)
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
			internalServerError(c, err)
			return
		}
		cl := &Collection{
			ID:          x.UID,
			ContentType: mdb.CONTENT_TYPE_REGISTRY.ByID[x.TypeID].Name,
			FilmDate:    Date{Time: props.FilmDate.Time},
		}

		// i18n - get from map by lang order
		for _, l := range LANG_ORDER[r.Language] {
			for _, i18n := range ci18ns {
				if i18n.Language == l {
					if cl.Name == "" && i18n.Name.Valid {
						cl.Name = i18n.Name.String
					}
					if cl.Description == "" && i18n.Description.Valid {
						cl.Description = i18n.Description.String
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
				internalServerError(c, err)
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

			cl.ContentUnits = append(cl.ContentUnits, u)
		}
		resp.Collections[i] = cl
	}

	c.JSON(http.StatusOK, resp)
}

func ContentUnitsHandler(c *gin.Context) {
	var r BaseRequest
	if c.Bind(&r) != nil {
		return
	}

	uid := c.Param("uid")
	db := c.MustGet("MDB_DB").(*sql.DB)
	cu, err := mdbmodels.ContentUnits(db, qm.Where("uid = ?", uid)).One()
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{})
			return
		} else {
			internalServerError(c, err)
			return
		}
	}

	var props mdb.ContentUnitProperties
	err = cu.Properties.Unmarshal(&props)
	if err != nil {
		internalServerError(c, err)
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
		internalServerError(c, err)
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
		qm.Where("content_unit_id = ?", cu.ID),
		qm.And("properties ->> 'url' is not null")).
		All()
	if err != nil {
		internalServerError(c, err)
		return
	}
	u.Files = make([]*File, len(files))
	for i, x := range files {
		var props mdb.FileProperties
		err := x.Properties.Unmarshal(&props)
		if err != nil {
			internalServerError(c, err)
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

		if x.FileCreatedAt.Valid {
			f.FilmDate = Date{Time: x.FileCreatedAt.Time}
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
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Can'd search for an empty text",
		})
		return
	}

	page := 0
	pageQ := c.Query("page")
	if pageQ != "" {
		var err error
		page, err = strconv.Atoi(pageQ)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status": "error",
				"error":  fmt.Sprintf("Illegal value provided for 'page' parameter: %s", pageQ),
			})
			return
		}
	}

	res, err := handleSearch(c.MustGet("ES_CLIENT").(*elastic.Client), "mdb_collections", text, page)
	if err != nil {
		internalServerError(c, err)
	}

	c.JSON(http.StatusOK, res)
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

func internalServerError(c *gin.Context, err error) {
	c.Error(err).SetType(gin.ErrorTypePrivate)
	c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "Internal Server Error"})
}
