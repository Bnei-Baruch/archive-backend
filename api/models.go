package api

import (
	"fmt"
	"strings"
	"time"
)

type BaseRequest struct {
	Language string `json:"language" form:"language" binding:"omitempty,len=2"`
}

type ListRequest struct {
	BaseRequest
	StartDate  *Date  `json:"start_date" form:"start_date" binding:"omitempty"`
	EndDate    *Date  `json:"end_date" form:"end_date" binding:"omitempty"`
	PageNumber int    `json:"page_no" form:"page_no" binding:"omitempty,min=1"`
	PageSize   int    `json:"page_size" form:"page_size" binding:"omitempty,min=1"`
	OrderBy    string `json:"order_by" form:"order_by" binding:"omitempty"`
}

type CollectionsRequest struct {
	ListRequest
	ContentType []string `json:"content_type" form:"content_type" binding:"omitempty"`
}

type ListResponse struct {
	Total int64 `json:"total"`
}

type CollectionsResponse struct {
	ListResponse
	Collections []*Collection `json:"collections"`
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
	FilmDate    Date   `json:"film_date"`
	Duration    int    `json:"duration,omitempty"`
	Language    string `json:"language,omitempty"`
	MimeType    string `json:"mimetype,omitempty"`
	Type        string `json:"type,omitempty"`
	SubType     string `json:"subtype,omitempty"`
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
