package es

import (
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type Collection struct {
	MDB_UID      string            `json:"mdb_uid"`
	ContentType  string            `json:"content_type"`
	FilmDate     *utils.Date       `json:"film_date"`
	Names        map[string]string `json:"names"`
	Descriptions map[string]string `json:"descriptions"`
	ContentUnits []*ContentUnit    `json:"content_units"`
}

type ContentUnit struct {
	MDB_UID          string      `json:"mdb_uid"`
	Name             string      `json:"name,omitempty"`
	Description      string      `json:"description,omitempty"`
	ContentType      string      `json:"content_type"`
	FilmDate         *utils.Date `json:"film_date,omitempty"`
	Duration         int16       `json:"duration,omitempty"`
	OriginalLanguage string      `json:"original_language,omitempty"`
	Translations     []string    `json:"translations,omitempty"`
	Tags             []string    `json:"tags,omitempty"`
	Sources          []string    `json:"sources,omitempty"`
	Authors          []string    `json:"authors,omitempty"`
	Persons          []string    `json:"persons,omitempty"`
	Transcript       string      `json:"transcript,omitempty"`
}

type File struct {
	MDB_UID  string      `json:"mdb_uid"`
	Name     string      `json:"name"`
	Size     int64       `json:"size"`
	URL      string      `json:"url"`
	Secure   int         `json:"secure"`
	FilmDate *utils.Date `json:"film_date"`
	Duration float64     `json:"duration,omitempty"`
	Language string      `json:"language,omitempty"`
	MimeType string      `json:"mimetype,omitempty"`
	Type     string      `json:"type,omitempty"`
	SubType  string      `json:"subtype,omitempty"`
}

type Classification struct {
	MDB_UID            string `json:"mdb_uid"`
	Name               string `json:"name,omitempty"`
	NameSuggest        string `json:"name_suggest,omitempty"`
	Description        string `json:"description,omitempty"`
	DescriptionSuggest string `json:"description_suggest,omitempty"`
	Type               string `json:"classification_type"`
}
