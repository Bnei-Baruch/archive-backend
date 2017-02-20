package models

import (
	"time"
)

type Lesson struct {
	KmediaModel
	FilmDate   time.Time        `json:"film_date,omitempty"`
	UserID     int              `json:"user_id,omitempty"`
	Containers []Container      `json:"containers,omitempty"`
}

type Container struct {
	KmediaModel
	Describable
	Name            string           `json:"name,omitempty"`
	FullDescription string           `json:"full_description,omitempty"`
	Filmdate        time.Time        `json:"filmdate,omitempty"`
	LecturerID      int              `json:"lecturer_id,omitempty"`
	Secure          int              `json:"secure"`
	MarkedForMerge  bool             `json:"marked_for_merge,omitempty"`
	SecureChanged   bool             `json:"secure_changed,omitempty"`
	AutoParsed      bool             `json:"auto_parsed,omitempty"`
	PlaytimeSecs    int              `json:"playtime_secs,omitempty"`
	UserID          int              `json:"user_id,omitempty"`
	ForCensorship   bool             `json:"for_censorship,omitempty"`
	OpenedByCensor  bool             `json:"opened_by_censor,omitempty"`
	ClosedByCensor  bool             `json:"closed_by_censor,omitempty"`
	CensorID        int              `json:"censor_id,omitempty"`
	Position        int              `json:"position,omitempty"`
	FileAssets      []FileAsset      `json:"file_assets,omitempty"`
}

type FileAsset struct {
	KmediaModel
	Describable
	Name         string             `json:"name,omitempty"`
	Lang         string             `json:"lang,omitempty"`
	AssetTypeID  string             `json:"asset_type_id,omitempty"`
	Date         time.Time          `json:"date,omitempty"`
	Size         int                `json:"size,omitempty"`
	ServerNameID string             `json:"server_name_id,omitempty"`
	Status       string             `json:"status,omitempty"`
	Lastuser     string             `json:"lastuser,omitempty"`
	Clicks       int                `json:"clicks,omitempty"`
	Secure       int                `json:"secure,omitempty"`
	PlaytimeSecs int                `json:"playtime_secs,omitempty"`
	UserID       int                `json:"user_id,omitempty"`
}

type KmediaModel struct {
	KmediaID  int                `json:"kmedia_id"`
	CreatedAt time.Time          `json:"created_at,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty"`
}

type Describable struct {
	Description string              `json:"description,omitempty"`
}
