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

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/queries"

	"github.com/Bnei-Baruch/archive-backend/utils"
)

var soffice string
var docFolder string

func ConvertDocx() {
	clock := Init()

	soffice = viper.GetString("elasticsearch.soffice-bin")
	if soffice == "" {
		panic("Soffice binary should be set in config.")
	}
	docFolder = path.Join(viper.GetString("elasticsearch.docx-folder"))
	utils.Must(os.MkdirAll(docFolder, 0777))

	utils.Must(convertDocx())

	Shutdown()
	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}

func convertDocx() error {
	var workersWG sync.WaitGroup
	docsCH := make(chan []string)
	workersWG.Add(1)
	var loadErr error
	var total uint64
	go func(wg *sync.WaitGroup) {
		defer close(docsCH)
		defer wg.Done()
		docs, err := loadDocs()
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
					err := downloadAndConvert(docBatch)
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

var sofficeMutex = &sync.Mutex{}

func downloadAndConvert(docBatch [][]string) error {
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
		docPath := path.Join(docFolder, docFilename)
		docxPath := path.Join(docFolder, docxFilename)
		if _, err := os.Stat(docxPath); !os.IsNotExist(err) {
			continue
		}
		if filepath.Ext(name) == ".doc" {
			convertDocs = append(convertDocs, docPath)
			//defer os.Remove(docPath)
		}

		// Download doc.
		resp, err := http.Get(fmt.Sprintf("https://cdn.kabbalahmedia.info/%s", uid))
		if err != nil {
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

	if len(convertDocs) > 0 {
		sofficeMutex.Lock()
		args := append([]string{"--headless", "--convert-to", "docx", "--outdir", docFolder}, convertDocs...)
		log.Infof("Command [%s]", strings.Join(args, " "))
		cmd := exec.Command(soffice, args...)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		sofficeMutex.Unlock()
		if _, ok := err.(*exec.ExitError); err != nil || ok {
			log.Warnf("soffice\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
			return errors.Wrapf(err, "Execute soffice")
		}
	}

	return nil
}

func loadDocs() ([][]string, error) {
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
