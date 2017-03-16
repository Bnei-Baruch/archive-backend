package mdb

import (
	"time"
)

type CollectionProperties struct {
	FilmDate Timestamp `json:"film_date"`
}

type ContentUnitProperties struct {
	FilmDate         Timestamp `json:"film_date"`
	Secure           int       `json:"secure"`
	OriginalLanguage string    `json:"original_language"`
	Duration         int       `json:"duration"`
}

type FileProperties struct {
	Secure   int    `json:"secure"`
	URL      string `json:"url"`
	Duration int    `json:"duration"`
}

// A time.Time like structure with support for date part only JSON marshalling
type Timestamp struct {
	time.Time
}

func (t *Timestamp) UnmarshalJSON(b []byte) error {
	err := t.Time.UnmarshalJSON(b)
	if err != nil {
		t.Time, err = time.Parse("\"2006-01-02\"", string(b))
		return err
	}
	return nil
}
