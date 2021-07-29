package mydb

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/volatiletech/null.v6"

	models2 "github.com/Bnei-Baruch/archive-backend/mydb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

const (
	SCAN_SIZE                 = 1000
	MAX_INTERVAL              = time.Duration(time.Minute)
	MIN_INTERVAL              = time.Duration(100 * time.Millisecond)
	CR_EVENT_TYPE_PLAYER_PLAY = "player-play"
	CR_EVENT_TYPE_PLAYER_STOP = "player-stop"
	WAIT_FOR_SAVE             = time.Duration(5 * time.Minute)
)

type Chronicles struct {
	ticker   *time.Ticker
	interval time.Duration
	evByAcc  map[string]*ChronicleEvent

	chroniclesUrl string
	lastReadId    string
	prevReadId    string
	nextRefresh   time.Time
}

type ScanResponse struct {
	Entries []*ChronicleEvent `json:"entries"`
}

type ChronicleEvent struct {
	AccountId       string      `json:"user_id"`
	CreatedAt       time.Time   `json:"created_at"`
	IPAddr          string      `boil:"ip_addr" json:"ip_addr" toml:"ip_addr" yaml:"ip_addr"`
	ID              string      `json:"id"`
	UserAgent       string      `json:"user_agent"`
	Namespace       string      `json:"namespace"`
	ClientEventID   null.String `json:"client_event_id,omitempty"`
	ClientEventType string      `json:"client_event_type"`
	ClientFlowID    null.String `json:"client_flow_id,omitempty"`
	ClientFlowType  null.String `json:"client_flow_type,omitempty"`
	ClientSessionID null.String `toml:"client_session_id"`
	Data            null.JSON   `json:"data,omitempty"`
	FirstScanAt     time.Time   `json:"-"`
}

type ChronicleEventData struct {
	UnitUID     string     `json:"unit_uid"`
	TimeZone    string     `json:"time_zone,omitempty"`
	CurrentTime null.Int64 `json:"current_time,omitempty"`
}

func (c *Chronicles) Run() {
	c.interval = MIN_INTERVAL
	c.ticker = time.NewTicker(MIN_INTERVAL)
	c.evByAcc = make(map[string]*ChronicleEvent, 0)

	db, err := sql.Open("postgres", viper.GetString("personal.db"))
	utils.Must(err)
	utils.Must(db.Ping())
	defer db.Close()

	h, err := models2.Histories(db, qm.OrderBy("chronicle_id")).One()
	if err == sql.ErrNoRows {
		c.lastReadId = ""
	} else if err != nil {
		utils.Must(err)
	} else {
		c.lastReadId = h.ChronicleID
	}
	c.chroniclesUrl = viper.GetString("personal.scan_url")
	go func() {
		refresh := func() {
			if err := c.refresh(); err != nil {
				log.Errorf("Error Refresh: %+v", err)
				panic(err)
			}
		}
		refresh()

		for range c.ticker.C {
			refresh()
		}
	}()
}

func (c *Chronicles) refresh() error {
	if c.nextRefresh.After(time.Now()) {
		return nil
	}
	entries, err := c.scanEvents()
	if err != nil {
		return err
	}
	if len(entries) == SCAN_SIZE {
		c.interval = maxDuration(c.interval/2, MIN_INTERVAL)
	} else {
		c.interval = minDuration(c.interval*2, MAX_INTERVAL)
	}
	c.nextRefresh = time.Now().Add(c.interval)
	return nil
}

func (c *Chronicles) scanEvents() ([]*ChronicleEvent, error) {

	log.Infof("Scanning chronicles entries, last successfull [%s]", c.lastReadId)
	b := bytes.NewBuffer([]byte(fmt.Sprintf(`{"id":"%s","limit":%d}`, c.lastReadId, SCAN_SIZE)))
	resp, err := http.Post(c.chroniclesUrl, "application/json", b)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("Response code %d for scan: %s.", resp.StatusCode, resp.Status))
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var scanResponse ScanResponse
	if err = json.Unmarshal(body, &scanResponse); err != nil {
		return nil, err
	}

	if len(scanResponse.Entries) > 0 {
		c.lastReadId = scanResponse.Entries[len(scanResponse.Entries)-1].ID
	}
	if err := c.saveEvents(scanResponse.Entries); err != nil {
		return nil, err
	}
	return scanResponse.Entries, nil
}

func (c *Chronicles) saveEvents(events []*ChronicleEvent) error {
	db, err := sql.Open("postgres", viper.GetString("personal.db"))
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

	for _, x := range events {
		if x.ClientEventType != CR_EVENT_TYPE_PLAYER_PLAY && x.ClientEventType != CR_EVENT_TYPE_PLAYER_STOP {
			continue
		}

		if prevE, ok := c.evByAcc[x.AccountId]; ok && prevE != nil {
			if x.ClientEventType == CR_EVENT_TYPE_PLAYER_STOP && x.FirstScanAt.After(c.nextRefresh.Add(WAIT_FOR_SAVE)) {
				c.evByAcc[x.AccountId] = nil
				continue
			}
			if x.ClientEventType != CR_EVENT_TYPE_PLAYER_PLAY {
				x.FirstScanAt = prevE.FirstScanAt
			}
		} else {
			x.FirstScanAt = c.nextRefresh
		}
		c.evByAcc[x.AccountId] = x
	}

	for _, x := range c.evByAcc {
		if x == nil {
			continue
		}
		if x.FirstScanAt.After(c.nextRefresh.Add(WAIT_FOR_SAVE)) {
			continue
		}
		c.evByAcc[x.AccountId] = nil
		err = c.insertEvent(tx, x)
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

func (c *Chronicles) insertEvent(tx *sql.Tx, ev *ChronicleEvent) error {
	var data map[string]interface{}
	if err := json.Unmarshal(ev.Data.JSON, &data); err != nil {
		return err
	}

	nParams := make(map[string]interface{})
	nParams["current_time"] = data["current_time"]

	j, err := json.Marshal(nParams)
	if err != nil {
		return err
	}

	year, month, day := ev.CreatedAt.Date()
	var tz string
	if v, ok := data["time_zone"]; ok {
		tz = fmt.Sprint(v)
	}
	timeZone, err := time.LoadLocation(tz)
	if err != nil {
		return err
	}
	sDay := time.Date(year, month, day, 0, 0, 0, 0, timeZone)
	eDay := sDay.Add(24 * time.Hour)
	log.Infof("%v, %v", sDay, eDay)
	unitUID := fmt.Sprint(data["unit_uid"])
	h, err := models2.Histories(tx,
		qm.Where("account_id = ? AND unit_uid = ?", ev.AccountId, unitUID),
		//qm.Where("account_id = ? AND unit_uid = ? AND created_at > ? AND  and created_at < ?", ev.AccountId, unitUID, sDay, eDay),
	).One()
	if err == sql.ErrNoRows {
		h = &models2.History{
			AccountID:   ev.AccountId[0:36],
			ChronicleID: ev.ID,
			UnitUID:     null.String{String: unitUID, Valid: true},
			Data:        null.JSON{JSON: j, Valid: true},
			CreatedAt:   ev.CreatedAt,
		}
		return h.Insert(tx)
	} else if err != nil {
		return err
	}

	params, err := margeData(h.Data, nParams)
	if err != nil {
		return err
	}
	h.Data = *params
	return h.Update(tx)
}

func margeData(data null.JSON, nd map[string]interface{}) (*null.JSON, error) {
	var d map[string]interface{}
	if err := json.Unmarshal(data.JSON, &d); err != nil {
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

func minDuration(x, y time.Duration) time.Duration {
	if x < y {
		return x
	}
	return y
}

func maxDuration(x, y time.Duration) time.Duration {
	if x >= y {
		return x
	}
	return y
}
