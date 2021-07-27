package mydb

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	"github.com/spf13/viper"
	"golang.org/x/text/unicode/cldr"
	"gopkg.in/volatiletech/null.v6"

	models2 "github.com/Bnei-Baruch/archive-backend/mydb/models"
)

type Chronicles struct {
	db *sql.DB
}

type ChronicleEvent struct {
	AccountId string             `json:"client_id"`
	Data      ChronicleEventData `json:"data"`
	CreatedAt time.Time          `json:"created_at"`
}

type ChronicleEventData struct {
	UnitUID     string             `json:"unit_uid"`
	TimeZone    cldr.TimeZoneNames `json:"time_zone,omitempty"`
	CurrentTime null.Int64         `json:"current_time,omitempty"`
	Json        null.String        `json:"data,omitempty"`
}

func (c *Chronicles) Run() {
	for true {
		//resp, err := http.Post("http://bbdev6.kbb1.com:9590/scan", "application/json", "{}")
		resp, err := http.Get("http://bbdev6.kbb1.com:9590/scan")
		if err != nil {
			c.Run()
			return
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			c.Run()
			return
		}

		if err := c.parseEvents(body); err != nil {
			c.Run()
			return
		}

		time.Sleep(time.Minute)
	}
}

func (c *Chronicles) parseEvents(b []byte) error {
	var d []*ChronicleEvent
	if err := json.Unmarshal(b, &d); err != nil {
		return err
	}

	db, err := sql.Open("postgres", viper.GetString("mdb.url"))
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, x := range d {
		err = c.addEvent(tx, x)
		if err != nil {
			break
		}
	}
	var eTx error
	if err == nil {
		eTx = tx.Commit()
	} else {
		eTx = tx.Rollback()
	}
	if eTx != nil {
		return eTx
	}
	return nil
}

func (c *Chronicles) addEvent(tx *sql.Tx, ev *ChronicleEvent) error {

	h, err := models2.Histories(tx, qm.Where("account_id = ? AND uid = ? and day > ", ev.AccountId, ev.Data.UnitUID)).One()
	if err != nil {
		return err
	}
	if h == nil {
		h = &models2.History{
			AccountID:   ev.AccountId,
			ChronicleID: "",
			UID:         null.String{String: ev.Data.UnitUID},
			Data:        null.JSON{},
			CreatedAt:   ev.CreatedAt,
		}
		return h.Insert(tx)
	}
	data, err := margeData(h.Data, ev.Data.Json.String)
	if err != nil {
		return err
	}
	h.Data = *data
	return h.Update(tx)
}

func margeData(data null.JSON, ndata string) (*null.JSON, error) {
	var d map[string]interface{}
	if err := json.Unmarshal(data.JSON, &d); err != nil {
		return nil, err
	}
	var nd map[string]interface{}
	if err := json.Unmarshal([]byte(ndata), &nd); err != nil {
		return nil, err
	}
	for k, v := range nd {
		d[k] = v
	}
	dStr, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return &null.JSON{JSON: dStr}, nil
}

func dayByTimeZone(t time.Time) {

}
