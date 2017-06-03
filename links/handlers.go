package links

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/vattle/sqlboiler/queries/qm"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

type FileBackendRequest struct {
	SHA1     string `json:"sha1"`
	Name     string `json:"name"`
	ClientIP string `json:"clientip"`
}

type FileBackendResponse struct {
	Url string `json:"url"`
}

var filerClient = &http.Client{
	Timeout: time.Second,
}

func FilesHandler(c *gin.Context) {
	db := c.MustGet("MDB_DB").(*sql.DB)

	uid := c.Param("uid")

	// jwplayer needs a known file extension, so we drop it here.
	uid = strings.Split(uid, ".")[0]

	file, err := mdbmodels.Files(db,
		qm.Select("sha1", "content_unit_id", "name"),
		qm.Where("uid = ?", uid)).
		One()
	if err != nil {
		if err == sql.ErrNoRows {
			c.AbortWithStatus(http.StatusNotFound)
			return
		} else {
			c.AbortWithError(http.StatusInternalServerError,
				errors.Wrap(err, "Lookup file in MDB")).
				SetType(gin.ErrorTypePrivate)
			return
		}
	}

	data := FileBackendRequest{
		SHA1:     hex.EncodeToString(file.Sha1.Bytes),
		Name:     file.Name,
		ClientIP: c.ClientIP(),
	}
	log.Infof("Handle file: %s %s", uid, data.SHA1)

	b := new(bytes.Buffer)
	err = json.NewEncoder(b).Encode(data)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError,
			errors.Wrap(err, "json.Encode")).
			SetType(gin.ErrorTypePrivate)
		return
	}

	url := viper.GetString("file_service.url1")
	req, err := http.NewRequest("POST", url, b)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError,
			errors.Wrap(err, "http.NewRequest")).
			SetType(gin.ErrorTypePrivate)
		return
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	res, err := filerClient.Do(req)
	var retry = false
	if err != nil {
		retry = true
		log.Errorf("Failed with first filer backend: %s", err)
	}
	if res.StatusCode >= http.StatusInternalServerError {
		retry = true
		log.Errorf("First filer backend crashed: [%d - %s] %s",
			res.StatusCode, http.StatusText(res.StatusCode), res.Status)
	}

	if retry {
		url = viper.GetString("file_service.url2")
		log.Infof("Retrying with second filer backend at %s", url)

		req, err = http.NewRequest("POST", url, b)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError,
				errors.Wrap(err, "http.NewRequest")).
				SetType(gin.ErrorTypePrivate)
			return
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")

		res, err = filerClient.Do(req)
		if err != nil {
			log.Errorf("Failed with second filer backend: %s", err)
			c.AbortWithError(http.StatusFailedDependency,
				errors.Wrap(err, "Failover filer backend communication error")).
				SetType(gin.ErrorTypePrivate)
			return
		}
		if res.StatusCode >= http.StatusInternalServerError {
			log.Errorf("Second filer backend crashed: [%d - %s] %s",
				res.StatusCode, http.StatusText(res.StatusCode), res.Status)
			c.AbortWithError(http.StatusFailedDependency,
				errors.Wrap(err, "Failover filer backend server error")).
				SetType(gin.ErrorTypePrivate)
			return
		}
	}

	if res.StatusCode == http.StatusNoContent {
		log.Infof("Filer backend no-content: %s %s", uid, data.SHA1)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	switch res.StatusCode {
	case http.StatusNoContent:
		log.Infof("Filer backend no-content: %s %s", uid, data.SHA1)
		c.AbortWithStatus(http.StatusNotFound)
		return
	case http.StatusOK:
		defer res.Body.Close()
		var body FileBackendResponse
		err = json.NewDecoder(res.Body).Decode(&body)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError,
				errors.Wrap(err, "json.Decode")).
				SetType(gin.ErrorTypePrivate)
			return
		}

		c.Redirect(http.StatusTemporaryRedirect, body.Url)
		return
	default:
		defer res.Body.Close()
		msg := fmt.Sprintf("Unknown filer backend status code [%d - %s] %s",
			res.StatusCode, http.StatusText(res.StatusCode), res.Status)
		log.Errorf(msg)
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorf("%s Error reading res.Body: %s", msg, err)
			c.AbortWithError(http.StatusInternalServerError, err).
				SetType(gin.ErrorTypePrivate)
			return
		}
		log.Errorf("res.Body: %s", b)
		c.AbortWithError(http.StatusInternalServerError, errors.Errorf(msg)).
			SetType(gin.ErrorTypePrivate)
		return
	}

}
