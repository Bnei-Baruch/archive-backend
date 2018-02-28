package mdb

import (
	"time"
)

type CollectionProperties struct {
	FilmDate        Timestamp `json:"film_date"`
	StartDate       Timestamp `json:"start_date"`
	EndDate         Timestamp `json:"end_date"`
	Country         string    `json:"country"`
	City            string    `json:"city"`
	FullAddress     string    `json:"full_address"`
	Genres          []string  `json:"genres"`
	DefaultLanguage string    `json:"default_language"`
	HolidayTag      string    `json:"holiday_tag"`
}

type ContentUnitProperties struct {
	FilmDate         Timestamp `json:"film_date"`
	Secure           int       `json:"secure"`
	OriginalLanguage string    `json:"original_language"`
	Duration         float64   `json:"duration"`
}

type FileProperties struct {
	Secure    int     `json:"secure"`
	URL       string  `json:"url"`
	Duration  float64 `json:"duration"`
	VideoSize string  `json:"video_size"`
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
