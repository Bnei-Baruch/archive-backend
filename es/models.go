package es

import (
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type EffectiveDate struct {
	EffectiveDate *utils.Date `json:"effective_date"`
}

type ResultType struct {
	ResultType string `json:"result_type"`
}

// For full description see make.py RESULTS TEMPLATE.
type Result struct {
	// Document type.
	ResultType string `json:"result_type"`

	IndexDate *utils.Date `json:"index_date,omitempty"`

	MDB_UID      string   `json:"mdb_uid"`
	TypedUids    []string `json:"typed_uids"`
	FilterValues []string `json:"filter_values"`

	// Result content fields.
	Title       string `json:"title"`
	FullTitle   string `json:"full_title"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`

	EffectiveDate *utils.Date `json:"effective_date,omitempty"`

	// Suggest field for autocomplete.
	TitleSuggest []string `json:"title_suggest"`
}
