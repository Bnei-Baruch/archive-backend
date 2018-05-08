package es

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Bnei-Baruch/sqlboiler/queries"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

func DownloadAndConvert(docBatch [][]string) error {
	log.Infof("DownloadAndConvert: %+v", docBatch)
	var convertDocs []string
	for _, docSource := range docBatch {
		uid := docSource[0]
		name := docSource[1]
		if filepath.Ext(name) != ".docx" && filepath.Ext(name) != ".doc" {
			log.Warnf("File type not supported %s %s, skipping.", uid, name)
			continue
		}

		docFilename := fmt.Sprintf("%s%s", uid, filepath.Ext(name))
		docxFilename := fmt.Sprintf("%s.docx", uid)
		folder, err := DocFolder()
		if err != nil {
			return err
		}
		docPath := path.Join(folder, docFilename)
		docxPath := path.Join(folder, docxFilename)
		if _, err := os.Stat(docxPath); !os.IsNotExist(err) {
			continue
		}
		if filepath.Ext(name) == ".doc" {
			convertDocs = append(convertDocs, docPath)
			//defer os.Remove(docPath)
		}

		// Download doc.
		resp, err := httpClient.Get(fmt.Sprintf("%s/%s", cdnUrl, uid))
		if err != nil {
			log.Warnf("Error downloading, Error: %+v", err)
			return err
		}
		if resp.StatusCode != 200 { // OK
			log.Warnf("Response code %d for %s, skip.", resp.StatusCode, uid)
			continue
		}

		out, err := os.Create(docPath)
		if err != nil {
			return errors.Wrapf(err, "os.Create %s", docPath)
		}

		_, err = io.Copy(out, resp.Body)

		if err := resp.Body.Close(); err != nil {
			log.Errorf("resp.Body.Close %s : %s", docPath, err.Error())
		}

		if err != nil {
			return errors.Wrapf(err, "io.Copy %s", docPath)
		}

		if err := out.Close(); err != nil {
			log.Errorf("out.Close %s : %s", docPath, err.Error())
		}
	}

	log.Infof("Converting: %+v", convertDocs)
	if len(convertDocs) > 0 {
		sofficeMutex.Lock()
		folder, err := DocFolder()
		if err != nil {
			return err
		}
		args := append([]string{"--headless", "--convert-to", "docx", "--outdir", folder}, convertDocs...)
		log.Infof("Command [%s]", strings.Join(args, " "))
		cmd := exec.Command(sofficeBin, args...)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
		sofficeMutex.Unlock()
		if err != nil {
			log.Errorf("soffice is '%s'. Error: %s", sofficeBin, err)
			log.Warnf("soffice\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
			return errors.Wrapf(err, "Execute soffice.")
		}
	}

	log.Info("DownloadAndConvert done.")
	return nil
}

// Will return empty string if no rows returned.
func LoadDocFilename(db *sql.DB, fileUID string) (string, error) {
	var fileName string
	err := queries.Raw(db, `
SELECT name
FROM files
WHERE name ~ '.docx?' AND
    language NOT IN ('zz', 'xx') AND
    content_unit_id IS NOT NULL AND
	secure=0 AND published IS TRUE
	AND uid = $1;`, fileUID).QueryRow().Scan(&fileName)

	if err == sql.ErrNoRows {
		return "", nil
	} else if err != nil {
		return "", errors.Wrapf(err, "LoadDocFilename - %s", fileUID)
	} else {
		return fileName, nil
	}
}

var sofficeMutex = &sync.Mutex{}

func loadDocs(db *sql.DB) ([][]string, error) {
	rows, err := queries.Raw(db, `
SELECT uid, name
FROM files
WHERE name ~ '.docx?' AND
    language NOT IN ('zz', 'xx') AND
    content_unit_id IS NOT NULL AND
    secure=0 AND published IS TRUE;`).Query()

	if err != nil {
		return nil, errors.Wrap(err, "Load docs")
	}
	defer rows.Close()

	return loadMap(rows)
}

func loadMap(rows *sql.Rows) ([][]string, error) {
	var m [][]string

	for rows.Next() {
		var uid string
		var name string
		err := rows.Scan(&uid, &name)
		if err != nil {
			return nil, errors.Wrap(err, "rows.Scan")
		}
		m = append(m, []string{uid, name})
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "rows.Err()")
	}

	return m, nil
}

func ConvertDocx(db *sql.DB) error {
	var workersWG sync.WaitGroup
	docsCH := make(chan []string)
	workersWG.Add(1)
	var loadErr error
	var total uint64
	go func(wg *sync.WaitGroup) {
		defer close(docsCH)
		defer wg.Done()
		docs, err := loadDocs(db)
		if err != nil {
			loadErr = errors.Wrap(err, "Fetch docs from mdb")
			return
		}
		log.Infof("%d docs in MDB", len(docs))
		total = uint64(len(docs))
		for _, doc := range docs {
			if len(doc) > 0 {
				docsCH <- doc
			} else {
				loadErr = errors.New("Empty doc, skipping. Should not happen.")
				return
			}
		}
	}(&workersWG)

	var done uint64 = 0
	var errs [5]error
	for i := 0; i < 5; i++ {
		workersWG.Add(1)
		go func(wg *sync.WaitGroup, i int) {
			defer wg.Done()
			for {
				var docBatch [][]string
				for j := 0; j < 50; j++ {
					doc := <-docsCH
					if len(doc) > 0 {
						docBatch = append(docBatch, doc)
					} else {
						break
					}
				}
				if len(docBatch) > 0 {
					err := DownloadAndConvert(docBatch)
					atomic.AddUint64(&done, uint64(len(docBatch)))
					if err != nil {
						errs[i] = err
						return
					}
					log.Infof("Done %d / %d", done, total)
				} else {
					log.Infof("Worker %d done.", i)
					return
				}
			}
		}(&workersWG, i)
	}

	workersWG.Wait()
	if loadErr != nil {
		return loadErr
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
