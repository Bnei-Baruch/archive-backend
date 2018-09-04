package es

import (
	"bytes"
	"context"
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
	"time"

	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/Bnei-Baruch/sqlboiler/queries"
	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

func DownloadAndConvert(docBatch [][]string) error {
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

		// all is good, file is here. Should we convert to docx first ?
		if filepath.Ext(name) == ".doc" {
			convertDocs = append(convertDocs, docPath)
		}
	}

	if len(convertDocs) > 0 {
		for _, docPath := range convertDocs {
			if _, err := os.Stat(docPath); os.IsNotExist(err) {
				return errors.Wrapf(err, "os.Stat %s", docPath)
			}
		}

		sofficeMutex.Lock()
		defer sofficeMutex.Unlock()
		folder, err := DocFolder()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(utils.MaxInt(15, len(convertDocs)))*time.Second)
		defer cancel()
		args := append([]string{"--headless", "--convert-to", "docx", "--outdir", folder}, convertDocs...)
		log.Infof("Command [%s]", strings.Join(args, " "))
		cmd := exec.CommandContext(ctx, sofficeBin, args...)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
		if ctx.Err() == context.DeadlineExceeded {
			log.Errorf("DeadlineExceeded! soffice is '%s'. Error: %s", sofficeBin, err)
			log.Warnf("soffice\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
			return errors.Wrapf(ctx.Err(), "DeadlineExceeded when executing soffice.")
		}
		if err != nil {
			log.Errorf("soffice is '%s'. Error: %s", sofficeBin, err)
			log.Warnf("soffice\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
			return errors.Wrapf(err, "Execute soffice.")
		} else {
			log.Infof("soffice successfully done.")
		}
	}
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
SELECT
  f.uid,
  f.name
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
	docs, err := loadDocs(db)
	if err != nil {
		return errors.Wrap(err, "Fetch docs from mdb")
	}
	total := len(docs)
	log.Infof("%d docs in MDB", total)

	var batch [][]string
	for i, doc := range docs {
		if len(doc) <= 0 {
			log.Warn("Empty doc, skipping. Should not happen.")
			continue
		}

		batch = append(batch, doc)

		if len(batch) == 50 {
			log.Infof("DownloadAndConvert %d / %d", i+1, total)
			if err := DownloadAndConvert(batch); err != nil {
				return errors.Wrapf(err, "DownloadAndConvert %d / %d", i, total)
			}
			batch = make([][]string, 0)
		}
	}

	// tail
	if len(batch) > 0 {
		log.Infof("DownloadAndConvert tail %d", len(batch))
		if err := DownloadAndConvert(batch); err != nil {
			return errors.Wrapf(err, "DownloadAndConvert tail %d", len(batch))
		}
	}

	return nil
}
