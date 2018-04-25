package events

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	log "github.com/Sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}

func CollectionCreate(d Data) {
	putToIndexer(indexer.CollectionUpdate, d.Payload["uid"].(string))
}

func CollectionDelete(d Data) {
	putToIndexer(indexer.CollectionUpdate, d.Payload["uid"].(string))
}

func CollectionUpdate(d Data) {
	putToIndexer(indexer.CollectionUpdate, d.Payload["uid"].(string))
}

func CollectionPublishedChange(d Data) {
	putToIndexer(indexer.CollectionUpdate, d.Payload["uid"].(string))
}

func CollectionContentUnitsChange(d Data) {
	putToIndexer(indexer.CollectionUpdate, d.Payload["uid"].(string))
}

func ContentUnitCreate(d Data) {
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func ContentUnitDelete(d Data) {
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func ContentUnitUpdate(d Data) {
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func ContentUnitPublishedChange(d Data) {
	uid := d.Payload["uid"].(string)
	putToIndexer(indexer.ContentUnitUpdate, uid)

	// Prepare unit thumbnail
	unit, err := mdbmodels.ContentUnitsG(qm.Where("uid=?", uid)).One()
	if err != nil {
		log.Errorf("Error loading unit from mdb %s: %s", uid, err.Error())
		return
	}

	ct := mdb.CONTENT_TYPE_REGISTRY.ByID[unit.TypeID].Name
	createThumbnail := unit.Published &&
		unit.Secure == 0 &&
		ct != consts.CT_KITEI_MAKOR &&
		ct != consts.CT_LELO_MIKUD &&
		ct != consts.CT_PUBLICATION &&
		ct != consts.CT_ARTICLE

	if createThumbnail {
		log.Infof("thumbnail %s [%s]", unit.UID, ct)
		AssetsAPI("thumbnail", unit.UID)
	}
}

func ContentUnitDerivativesChange(d Data) {
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func ContentUnitSourcesChange(d Data) {
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func ContentUnitTagsChange(d Data) {
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func ContentUnitPersonsChange(d Data) {
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func ContentUnitPublishersChange(d Data) {
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func FilePublished(d Data) {
	putToIndexer(indexer.FileUpdate, d.Payload["uid"].(string))

	uid := d.Payload["uid"].(string)
	file, err := mdbmodels.FilesG(qm.Where("uid=?", uid)).One()
	if err != nil {
		log.Errorf("Error loading file from mdb %s: %s", uid, err.Error())
		return
	}

	if file.Secure == 0 {
		switch file.Type {
		case "image":
			if strings.HasSuffix(file.Name, ".zip") {
				log.Infof("unzip %s [%s]", file.Name, file.UID)
				AssetsAPI("unzip", file.UID)
			}

		case "text":
			if file.MimeType.String == "application/msword" {
				log.Infof("doc2html %s [%s]", file.Name, file.UID)
				AssetsAPI("doc2html", file.UID)
			}
		}
	}
}

func FileReplace(d Data) {
	oFile := d.Payload["old"].(map[string]interface{})
	nFile := d.Payload["new"].(map[string]interface{})

	putToIndexer(indexer.FileUpdate, oFile["uid"].(string))
	putToIndexer(indexer.FileUpdate, nFile["uid"].(string))
}

func FileInsert(d Data) {
	putToIndexer(indexer.FileUpdate, d.Payload["uid"].(string))
}

func FileUpdate(d Data) {
	putToIndexer(indexer.FileUpdate, d.Payload["uid"].(string))

	RemoveFile(d.Payload["uid"].(string))
}

func SourceCreate(d Data) {
	putToIndexer(indexer.SourceUpdate, d.Payload["uid"].(string))
}

func SourceUpdate(d Data) {
	putToIndexer(indexer.SourceUpdate, d.Payload["uid"].(string))
}

func TagCreate(d Data) {
	putToIndexer(indexer.TagUpdate, d.Payload["uid"].(string))
}

func TagUpdate(d Data) {
	putToIndexer(indexer.TagUpdate, d.Payload["uid"].(string))
}

func PersonCreate(d Data) {
	putToIndexer(indexer.PersonUpdate, d.Payload["uid"].(string))
}

func PersonDelete(d Data) {
	putToIndexer(indexer.PersonUpdate, d.Payload["uid"].(string))
}

func PersonUpdate(d Data) {
	putToIndexer(indexer.PersonUpdate, d.Payload["uid"].(string))
}

func PublisherCreate(d Data) {
	putToIndexer(indexer.PublisherUpdate, d.Payload["uid"].(string))
}

func PublisherUpdate(d Data) {
	putToIndexer(indexer.PublisherUpdate, d.Payload["uid"].(string))
}

func putToIndexer(f func(string) error, s string) {
	indexerQueue.Enqueue(IndexerTask{F: f, S: s})
}

// AssetsAPI sends request to unzip api by file UID
func AssetsAPI(path string, uid string) {
	apiURL := viper.GetString("assets_service.url")
	resp, err := httpClient.Get(fmt.Sprintf("%s%s/%s", apiURL, path, uid))
	if err != nil {
		log.Errorf("file_service http.POST path= %s uid = %s: %s", path, uid, err.Error())
		return
	}

	if resp.StatusCode != 200 {
		log.Warnf("assets_service returned status code %d path= %s uid = %s", resp.StatusCode, path, uid)
	}
}

type FileBackendRequest struct {
	SHA1 string `json:"sha1"`
	Name string `json:"name"`
}

// RemoveFile to send post req to file-api and remove file from search?
func RemoveFile(uid string) {
	file, err := mdbmodels.FilesG(qm.Where("uid=?", uid)).One()
	if err != nil {
		log.Errorf("Error loading file from mdb %s: %s", uid, err.Error())
		return
	}

	if file.Secure > 0 && file.Published == true {
		log.Infof("file_service disable file %s", uid)

		data := FileBackendRequest{
			SHA1: hex.EncodeToString(file.Sha1.Bytes),
			Name: file.Name,
		}

		b := new(bytes.Buffer)
		err := json.NewEncoder(b).Encode(data)
		if err != nil {
			log.Errorf("json.Encode uid = %s: %s", uid, err.Error())
			return
		}

		apiURL := viper.GetString("file_service.url1") + "/api/v1/getremove"
		resp, err := httpClient.Post(apiURL, "application/json; charset=utf-8", b)
		if err != nil {
			log.Errorf("file_service http.POST uid = %s: %s", uid, err.Error())
			return
		}

		if resp.StatusCode != 200 {
			log.Warnf("file_service returned status code %d", resp.StatusCode)
		}
	}
}
