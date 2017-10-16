package utils

import (
	"fmt"
	"strings"
	"time"
)

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

func (d *Date) Scan(value interface{}) error {
	var err error
	d.Time, err = time.Parse("2006-01-02", value.(string))
	return err
}
