package es

import (
	"strconv"
	"time"

	"github.com/Bnei-Baruch/archive-backend/mdb"
)

type Collection struct {
	MDB_UID      string            `json:"mdb_uid"`
	ContentType  string            `json:"content_type"`
	FilmDate     time.Time         `json:"film_date"`
	Names        map[string]string `json:"names"`
	Descriptions map[string]string `json:"descriptions"`
	ContentUnits ContentUnits      `json:"content_units"`
}

type ContentUnit struct {
	MDB_UID          string            `json:"mdb_uid"`
	ContentType      string            `json:"content_type"`
	NameInCollection string            `json:"name_in_collection"`
	FilmDate         time.Time         `json:"film_date"`
	Secure           int               `json:"secure"`
	Duration         float64           `json:"duration,omitempty"`
	OriginalLanguage string            `json:"original_language,omitempty"`
	Names            map[string]string `json:"names"`
	Descriptions     map[string]string `json:"descriptions"`
	Files            []*File           `json:"files"`
}

type File struct {
	MDB_UID  string    `json:"mdb_uid"`
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	URL      string    `json:"url"`
	Secure   int       `json:"secure"`
	FilmDate time.Time `json:"film_date"`
	Duration float64   `json:"duration,omitempty"`
	Language string    `json:"language,omitempty"`
	MimeType string    `json:"mimetype,omitempty"`
	Type     string    `json:"type,omitempty"`
	SubType  string    `json:"subtype,omitempty"`
}

// Sort helpers
// See https://golang.org/pkg/sort/
type ContentUnits []*ContentUnit

func (s ContentUnits) Len() int      { return len(s) }
func (s ContentUnits) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type ByNameInCollection struct{ ContentUnits }

func (s ByNameInCollection) Less(i, j int) bool {
	a, b := s.ContentUnits[i], s.ContentUnits[j]

	// Lesson parts should be sorted by numerically
	if a.ContentType == mdb.CT_LESSON_PART && b.ContentType == mdb.CT_LESSON_PART {
		ai, err := strconv.Atoi(a.NameInCollection)
		if err != nil {
			bi, err := strconv.Atoi(b.NameInCollection)
			if err != nil {
				return ai < bi
			}
		}
	}

	return a.NameInCollection < b.NameInCollection
}
