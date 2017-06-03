package links

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/vattle/sqlboiler/queries/qm"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"strings"
)

type FileBackendRequest struct {
	SHA1     string `json:"sha1"`
	Name     string `json:"name"`
	ClientIP string `json:"clientip"`
}

type FileBackendResponse struct {
	Url string `json:"url"`
}

func FilesHandler(c *gin.Context) {
	db := c.MustGet("MDB_DB").(*sql.DB)
	uid := c.Param("uid")
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

	log.Infof("Handle file: %s", file)

	data := FileBackendRequest{
		SHA1:     hex.EncodeToString(file.Sha1.Bytes),
		Name:     file.Name,
		ClientIP: c.ClientIP(),
	}

	b := new(bytes.Buffer)
	err = json.NewEncoder(b).Encode(data)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError,
			errors.Wrap(err, "json.Encode")).
			SetType(gin.ErrorTypePrivate)
		return
	}

	url := viper.GetString("file_service.url1")
	log.Infof("file_service.url: %s", url)

	res, _ := http.Post(url, "application/json; charset=utf-8", b)
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
}
