package api

import (
	"fmt"
	"gopkg.in/nullbio/null.v6"
	"strings"
	"time"
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

type ContentTypesFilter struct {
	ContentTypes []string `json:"content_types" form:"content_type" binding:"omitempty"`
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
	}

	return s, e, err
}

type CollectionsRequest struct {
	ListRequest
	ContentTypesFilter
	DateRangeFilter
}

type CollectionsResponse struct {
	ListResponse
	Collections []*Collection `json:"collections"`
}

type HierarchyRequest struct {
	BaseRequest
	RootUID  string `json:"root" form:"root" binding:"omitempty,len=8"`
	Depth    int    `json:"depth" form:"depth"`
}

func NewCollectionsResponse() *CollectionsResponse {
	return &CollectionsResponse{Collections: make([]*Collection, 0)}
}

type Collection struct {
	ID           string         `json:"id"`
	ContentType  string         `json:"content_type"`
	FilmDate     Date           `json:"film_date"`
	Name         string         `json:"name,omitempty"`
	Description  string         `json:"description,omitempty"`
	ContentUnits []*ContentUnit `json:"content_units"`
}

type ContentUnit struct {
	ID               string  `json:"id"`
	ContentType      string  `json:"content_type"`
	NameInCollection string  `json:"name_in_collection,omitempty"`
	FilmDate         Date    `json:"film_date"`
	Name             string  `json:"name,omitempty"`
	Description      string  `json:"description,omitempty"`
	Duration         int     `json:"duration,omitempty"`
	OriginalLanguage string  `json:"original_language,omitempty"`
	Files            []*File `json:"files,omitempty"`
}

type File struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
	DownloadURL string `json:"download_url"`
	Duration    int    `json:"duration,omitempty"`
	Language    string `json:"language,omitempty"`
	MimeType    string `json:"mimetype,omitempty"`
	Type        string `json:"type,omitempty"`
	SubType     string `json:"subtype,omitempty"`
}

type Source struct {
	UID         string      `json:"uid"`
	Pattern     null.String `json:"pattern,omitempty"`
	Type        string      `json:"type"`
	Name        null.String `json:"name"`
	Description null.String `json:"description,omitempty"`
	Children    []*Source   `json:"children,omitempty"`
	ID          int64       `json:"-"`
	ParentID    null.Int64  `json:"-"`
	Position    null.Int    `json:"-"`
}

type Author struct {
	Code     string      `json:"code"`
	Name     string      `json:"name"`
	FullName null.String `json:"full_name,omitempty"`
	Children []*Source   `json:"children,omitempty"`
}

type Tag struct {
	UID      string      `json:"uid"`
	Pattern  null.String `json:"pattern,omitempty"`
	Label    null.String `json:"label"`
	Children []*Tag      `json:"children,omitempty"`
	ID       int64       `json:"-"`
	ParentID null.Int64  `json:"-"`
}

// Custom fields

// A time.Time like structure with date part only JSON marshalling
type Date struct {
	time.Time
}

func (d *Date) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", d.Time.Format("2006-01-02"))), nil
}

func (d *Date) UnmarshalJSON(b []byte) error {
	var err error
	d.Time, err = time.Parse("2006-01-02", strings.Trim(string(b), "\""))
	return err
}
