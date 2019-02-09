package es

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Bnei-Baruch/sqlboiler/queries"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
)

var httpClient = &http.Client{
	Timeout: 600 * time.Second,
}

func DocText(uid string) (string, error) {
	resp, err := httpClient.Get(fmt.Sprintf("%s/doc2text/%s", unzipUrl, uid))
	if err != nil {
		log.Warnf("Error preparing docs, Error: %+v", err)
		return "", err
	}
	if resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		bodyString := string(bodyBytes)
		return bodyString, nil
	} else {
		log.Warnf("Response code %d for %s, skip.", resp.StatusCode, uid)
		return "", nil
	}
}

type unzipResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func Prepare(uids []string) (error, bool, map[string]int) {
	successMap := make(map[string]int)
	for _, uid := range uids {
		successMap[uid] = -1
	}
	// Prepare docs batch.
	log.Infof("Preparing %s docs to docx.", strings.Join(uids, ","))
	resp, err := httpClient.Get(fmt.Sprintf("%s/prepare/%s", unzipUrl, strings.Join(uids, ",")))
	log.Info("Finished Trying Get.")
	if err != nil {
		log.Warnf("Error preparing docs, Error: %+v.", err)
		return err, false, successMap
	}
	if resp.StatusCode != http.StatusOK { // OK
		log.Errorf("Response code %d for %s, error.", resp.StatusCode, strings.Join(uids, ","))
		return err, false, successMap
	}
	body, err := ioutil.ReadAll(resp.Body)
	log.Info("Finished reading body.")
	if err != nil {
		log.Error("Could not read response body.")
		return err, false, successMap
	}

	var data []unzipResponse
	json.Unmarshal(body, &data)

	if len(data) != len(uids) {
		return errors.New(fmt.Sprintf("Response length is not as request uids length. Expected %d, got %d", len(uids), len(data))), false, successMap
	}

	backoff := false
	var errors []string
	log.Infof("Ranging over data: %v", data)
	for i, internalResponse := range data {
		successMap[uids[i]] = internalResponse.Code
		if internalResponse.Code == http.StatusServiceUnavailable {
			// Backoff
			backoff = true
		} else if internalResponse.Code != http.StatusOK {
			// Don't repeat request, continue.
			errors = append(errors, internalResponse.Message)
		}
	}
	if len(errors) > 0 {
		log.Warn(strings.Join(errors, ","))
	}
	if backoff {
		log.Info("Successfully done, backoff: true.")
		return nil, true, successMap
	} else {
		log.Info("Successfully done, backoff: false.")
		return nil, false, successMap
	}
}

func loadDocs(db *sql.DB) ([]string, error) {
	rows, err := queries.Raw(db, `
SELECT
  f.uid
FROM files f
  INNER JOIN content_units cu ON f.content_unit_id = cu.id
                                 AND f.name ~ '.docx?$'
                                 AND f.language NOT IN ('zz', 'xx')
                                 AND f.secure = 0
                                 AND f.published IS TRUE
                                 AND cu.secure = 0
                                 AND cu.published IS TRUE
                                 AND cu.type_id != 42;`).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load docs")
	}
	defer rows.Close()

	return loadMap(rows)
}

func loadMap(rows *sql.Rows) ([]string, error) {
	var m []string

	for rows.Next() {
		var uid string
		err := rows.Scan(&uid)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		m = append(m, uid)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Err()")
	}

	return m, nil
}

func ConvertDocx(db *sql.DB) error {
	docs, err := loadDocs(db)
	if err != nil {
		return errors.Wrap(err, "Fetch docs from mdb")
	}
	total := len(docs)
	log.Infof("%d docs in MDB", total)

	var notEmptyDocs []string
	for _, doc := range docs {
		if len(doc) <= 0 {
			log.Warn("Empty doc, skipping. Should not happen.")
			continue
		}
		notEmptyDocs = append(notEmptyDocs, doc)
	}

	batches := make(chan []string)
	batchSize := prepareDocsBatchSize

	go func(notEmptyDocs []string, batches chan []string) {
		for start := 0; start < len(notEmptyDocs); start += batchSize {
			end := start + batchSize
			if end > len(notEmptyDocs) {
				end = len(notEmptyDocs)
			}
			batches <- notEmptyDocs[start:end]
		}
		close(batches)
	}(notEmptyDocs, batches)

	parallelism := prepareDocsParallelism
	var waitDone sync.WaitGroup
	waitDone.Add(parallelism)
	batchesDone := 0

	prepareMutex := &sync.Mutex{}
	prepareErr := error(nil)
	successMap := make(map[string]int)
	for i := 0; i < parallelism; i++ {
		go func(j int, batches chan []string) {
			for batch := range batches {
				prepareMutex.Lock()
				batchesDone += 1
				currentBatchDone := batchesDone
				prepareMutex.Unlock()
				log.Infof("[%d] Prepare %d / %d", j, currentBatchDone, len(notEmptyDocs)/batchSize)
				sleep := 0 * time.Second
				tryRetry := true
				retries := 5
				for ; retries > 0 && tryRetry; retries-- {
					log.Infof("Retry[%d]: %d, tryRetry: %t", j, retries, tryRetry)
					if sleep > 0 {
						log.Infof("Bakoff[%d], sleep %.2f, retry: %d", j, sleep.Seconds(), 5-retries)
						time.Sleep(sleep)
					}
					var batchSuccessMap map[string]int
					// ERR SHOULD BE LAST
					err, tryRetry, batchSuccessMap = Prepare(batch)
					if tryRetry {
						log.Infof("Try retry[%d]: true", j)
					} else {
						log.Infof("Try retry[%d]: false", j)
					}
					shouldBreak := false
					nextBatch := []string{}
					prepareMutex.Lock()
					for uid, code := range batchSuccessMap {
						currentCode, ok := successMap[uid]
						if ok {
							if currentCode != http.StatusOK {
								successMap[uid] = code
							} else if code != http.StatusOK {
								errStr := fmt.Sprintf("Making things worse, had %d for uid %s not got %d.", currentCode, uid, code)
								log.Error(errStr)
								if prepareErr == nil {
									prepareErr = errors.New(errStr)
								}
							}
						} else {
							successMap[uid] = code
						}
						if currentCode != http.StatusOK {
							nextBatch = append(nextBatch, uid)
						}
					}
					reason := ""
					if prepareErr != nil {
						shouldBreak = true
						reason = prepareErr.Error()
					}
					prepareMutex.Unlock()
					if shouldBreak {
						log.Errorf("Breaking[%d]... Due to: %s.", j, reason)
						break
					}
					if err != nil {
						log.Infof("Error while Prepare %d / %d. Error: %s", currentBatchDone, len(notEmptyDocs)/batchSize, err)
						prepareMutex.Lock()
						if prepareErr != nil {
							prepareErr = err
						}
						prepareMutex.Unlock()
						break
					}
					if tryRetry {
						log.Infof("Trying to retry [%d].", j)
						if sleep == 0 {
							sleep = 10 * time.Second
						} else {
							sleep += 10 * time.Second
						}
					} else {
						log.Infof("Trying not to retry [%d]. Retries: %d.", j, retries)
					}
					// At next retry, we want to try only failed uids.
					batch = nextBatch
				}
				shouldBreak := false
				reason := ""
				log.Infof("[%d] Locking...", j)
				prepareMutex.Lock()
				if prepareErr != nil {
					reason = prepareErr.Error()
					shouldBreak = true
				} else if retries == 0 {
					prepareErr = errors.New(fmt.Sprintf("No more retries[%d]. Exiting.", j))
					reason = prepareErr.Error()
					shouldBreak = true
				}
				prepareMutex.Unlock()
				log.Infof("[%d] Unlocking...", j)
				if shouldBreak {
					log.Errorf("Breaking... Due to: %s.", reason)
					break
				}
			}
			log.Infof("[%d] Done", j)
			waitDone.Done()
		}(i, batches)
	}
	waitDone.Wait()

	reverseSuccessMap := make(map[int]int)
	for _, code := range successMap {
		if _, ok := reverseSuccessMap[code]; ok {
			reverseSuccessMap[code]++
		} else {
			reverseSuccessMap[code] = 1
		}
	}
	for code, count := range reverseSuccessMap {
		log.Infof("Code: %d Count: %d.", code, count)
	}
	return nil
}
