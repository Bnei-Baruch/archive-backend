package api

import (
	"time"

	"gopkg.in/volatiletech/null.v6"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

type BaseRequest struct {
	Language string `json:"language" form:"language" binding:"omitempty,len=2"`
}

type ListRequest struct {
	BaseRequest
	PageNumber int    `json:"page_no" form:"page_no" binding:"omitempty,min=1"`
	PageSize   int    `json:"page_size" form:"page_size" binding:"omitempty,min=1"`
	StartIndex int    `json:"start_index" form:"start_index" binding:"omitempty,min=1"`
	StopIndex  int    `json:"stop_index" form:"stop_index" binding:"omitempty,min=1"`
	OrderBy    string `json:"order_by" form:"order_by" binding:"omitempty"`
}

type ListResponse struct {
	Total int64 `json:"total"`
}

type ItemRequest struct {
	BaseRequest
	UID string
}

type IDsFilter struct {
	IDs []string `json:"ids" form:"id" binding:"omitempty"`
}

type ContentTypesFilter struct {
	ContentTypes []string `json:"content_types" form:"content_type" binding:"omitempty"`
}

type SourcesFilter struct {
	Authors []string `json:"authors" form:"author" binding:"omitempty"`
	Sources []string `json:"sources" form:"source" binding:"omitempty,dive,len=8"`
}

type TagsFilter struct {
	Tags []string `json:"tags" form:"tag" binding:"omitempty,dive,len=8"`
}

type DateRangeFilter struct {
	StartDate string `json:"start_date" form:"start_date" binding:"omitempty"`
	EndDate   string `json:"end_date" form:"end_date" binding:"omitempty"`
}

func (drf *DateRangeFilter) Range() (time.Time, time.Time, error) {
	var err error
	var s, e time.Time

	if drf.StartDate != "" {
		s, err = time.Parse("2006-01-02", drf.StartDate)
	}
	if err == nil && drf.EndDate != "" {
		e, err = time.Parse("2006-01-02", drf.EndDate)
		if err == nil {
			e = e.Add(24*time.Hour - 1) // make the hour 23:59:59.999999999
		}
	}

	return s, e, err
}

type GenresProgramsFilter struct {
	Genres   []string `json:"genres" form:"genre" binding:"omitempty"`
	Programs []string `json:"programs" form:"program" binding:"omitempty"`
}

type CollectionsFilter struct {
	Collections []string `json:"collections" form:"collection" binding:"omitempty"`
}

type CollectionsRequest struct {
	ListRequest
	IDsFilter
	ContentTypesFilter
	DateRangeFilter
	WithUnits bool `json:"with_units" form:"with_units"`
}

type CollectionsResponse struct {
	ListResponse
	Collections []*Collection `json:"collections"`
}

type ContentUnitsRequest struct {
	ListRequest
	IDsFilter
	ContentTypesFilter
	DateRangeFilter
	SourcesFilter
	TagsFilter
	GenresProgramsFilter
	CollectionsFilter
}

type ContentUnitsResponse struct {
	ListResponse
	ContentUnits []*ContentUnit `json:"content_units"`
}

type LessonsRequest struct {
	ListRequest
	DateRangeFilter
	SourcesFilter
	TagsFilter
}

type HierarchyRequest struct {
	BaseRequest
	RootUID string `json:"root" form:"root" binding:"omitempty,len=8"`
	Depth   int    `json:"depth" form:"depth"`
}

func NewCollectionsResponse() *CollectionsResponse {
	return &CollectionsResponse{Collections: make([]*Collection, 0)}
}

func NewContentUnitsResponse() *ContentUnitsResponse {
	return &ContentUnitsResponse{ContentUnits: make([]*ContentUnit, 0)}
}

type Collection struct {
	ID              string         `json:"id"`
	ContentType     string         `json:"content_type"`
	Name            string         `json:"name,omitempty"`
	Description     string         `json:"description,omitempty"`
	FilmDate        *utils.Date    `json:"film_date,omitempty"`
	StartDate       *utils.Date    `json:"start_date,omitempty"`
	EndDate         *utils.Date    `json:"end_date,omitempty"`
	Country         string         `json:"country,omitempty"`
	City            string         `json:"city,omitempty"`
	FullAddress     string         `json:"full_address,omitempty"`
	Genres          []string       `json:"genres,omitempty"`
	DefaultLanguage string         `json:"default_language,omitempty"`
	HolidayID       string         `json:"holiday_id,omitempty"`
	ContentUnits    []*ContentUnit `json:"content_units,omitempty"`
}

type ContentUnit struct {
	ID               string                  `json:"id"`
	ContentType      string                  `json:"content_type"`
	NameInCollection string                  `json:"name_in_collection,omitempty"`
	FilmDate         *utils.Date             `json:"film_date,omitempty"`
	Name             string                  `json:"name,omitempty"`
	Description      string                  `json:"description,omitempty"`
	Duration         float64                 `json:"duration,omitempty"`
	OriginalLanguage string                  `json:"original_language,omitempty"`
	Files            []*File                 `json:"files,omitempty"`
	Collections      map[string]*Collection  `json:"collections,omitempty"`
	Sources          []string                `json:"sources,omitempty"`
	Tags             []string                `json:"tags,omitempty"`
	SourceUnits      map[string]*ContentUnit `json:"source_units,omitempty"`
	DerivedUnits     map[string]*ContentUnit `json:"derived_units,omitempty"`
}

type File struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Duration float64 `json:"duration,omitempty"`
	Language string  `json:"language,omitempty"`
	MimeType string  `json:"mimetype,omitempty"`
	Type     string  `json:"type,omitempty"`
	SubType  string  `json:"subtype,omitempty"`
}

type Source struct {
	UID         string      `json:"id"`
	ParentUID   string      `json:"parent_id"`
	Type        string      `json:"type"`
	Name        null.String `json:"name"`
	Description null.String `json:"description,omitempty"`
	Children    []*Source   `json:"children,omitempty"`
	ID          int64       `json:"-"`
	ParentID    null.Int64  `json:"-"`
	Position    null.Int    `json:"-"`
}

type Author struct {
	Code     string      `json:"id"`
	Name     string      `json:"name"`
	FullName null.String `json:"full_name,omitempty"`
	Children []*Source   `json:"children,omitempty"`
}

type Tag struct {
	UID       string      `json:"id"`
	ParentUID string      `json:"parent_id"`
	Label     null.String `json:"label"`
	Children  []*Tag      `json:"children,omitempty"`
	ID        int64       `json:"-"`
	ParentID  null.Int64  `json:"-"`
}

type CollectionUpdateStatus struct {
	UID        string     `json:"id"`
	LastUpdate utils.Date `json:"last_update"`
	UnitsCount int        `json:"units_count"`
}
