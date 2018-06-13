package es

import (
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

type EffectiveDate struct {
	EffectiveDate *utils.Date `json:"effective_date"`
}

// For full description see make.py RESULTS TEMPLATE.
type Result struct {
	// Document type.
	ResultType string `json:"result_type"`

	MDB_UID      string   `json:"mdb_uid"`
	TypedUIDs    []string `json:"typed_uids"`
	FilterValues []string `json:"filter_values"`

	// Result content fields.
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`

	EffectiveDate *utils.Date `json:"effective_date,omitempty"`
}

type ClassificationIntent struct {
	MDB_UID        string                    `json:"mdb_uid"`
	Name           string                    `json:"name"`
	ContentType    string                    `json:"content_type"`
	Exist          bool                      `json:"exist"`
	Score          *float64                  `json:"score,omitempty"`
	Explanation    elastic.SearchExplanation `json:"explanation,omitempty"`
	MaxScore       *float64                  `json:"max_score,omitempty"`
	MaxExplanation elastic.SearchExplanation `json:"max_explanation,omitempty"`
}

type Collection struct {
	MDB_UID                  string      `json:"mdb_uid"`
	TypedUIDs                []string    `json:"typed_uids"`
	Name                     string      `json:"name"`
	Description              string      `json:"description"`
	ContentType              string      `json:"content_type"`
	ContentUnitsContentTypes []string    `json:"content_units_content_types,omitempty"`
	EffectiveDate            *utils.Date `json:"effective_date"`
	OriginalLanguage         string      `json:"original_language,omitempty"`
}

type ContentUnit struct {
	MDB_UID                 string      `json:"mdb_uid"`
	TypedUIDs               []string    `json:"typed_uids"`
	Name                    string      `json:"name,omitempty"`
	Description             string      `json:"description,omitempty"`
	ContentType             string      `json:"content_type"`
	CollectionsContentTypes []string    `json:"collections_content_types,omitempty"`
	EffectiveDate           *utils.Date `json:"effective_date,omitempty"`
	Duration                uint64      `json:"duration,omitempty"`
	OriginalLanguage        string      `json:"original_language,omitempty"`
	Translations            []string    `json:"translations,omitempty"`
	Tags                    []string    `json:"tags,omitempty"`
	Sources                 []string    `json:"sources,omitempty"`
	Authors                 []string    `json:"authors,omitempty"`
	Persons                 []string    `json:"persons,omitempty"`
	Transcript              string      `json:"transcript,omitempty"`
}

type File struct {
	MDB_UID  string      `json:"mdb_uid"`
	Name     string      `json:"name"`
	Size     uint64      `json:"size"`
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

type Source struct {
	MDB_UID string `json:"mdb_uid"`
	Name    string `json:"name"`

	// Deprecated fields (since we use 'Result Template' in order to index the sources):
	Description string   `json:"description"`
	Content     string   `json:"content"`
	Sources     []string `json:"sources"`
	Authors     []string `json:"authors"`
	PathNames   []string `json:"path_names"`
	FullName    []string `json:"full_name"`
}
