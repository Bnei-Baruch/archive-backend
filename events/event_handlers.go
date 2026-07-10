package events

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/common"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}

func CollectionCreate(e Event) {
	putToIndexer(indexer.CollectionUpdate, e.Payload["uid"].(string))
}

func CollectionDelete(e Event) {
	putToIndexer(indexer.CollectionUpdate, e.Payload["uid"].(string))
}

func CollectionUpdate(e Event) {
	putToIndexer(indexer.CollectionUpdate, e.Payload["uid"].(string))
}

func CollectionPublishedChange(e Event) {
	putToIndexer(indexer.CollectionUpdate, e.Payload["uid"].(string))
}

func CollectionContentUnitsChange(e Event) {
	putToIndexer(indexer.CollectionUpdate, e.Payload["uid"].(string))
}

func ContentUnitCreate(e Event) {
	putToIndexer(indexer.ContentUnitUpdate, e.Payload["uid"].(string))
}

func ContentUnitDelete(e Event) {
	putToIndexer(indexer.ContentUnitUpdate, e.Payload["uid"].(string))
}

func ContentUnitUpdate(e Event) {
	putToIndexer(indexer.ContentUnitUpdate, e.Payload["uid"].(string))
}

func ContentUnitPublishedChange(e Event) {
	uid := e.Payload["uid"].(string)
	putToIndexer(indexer.ContentUnitUpdate, uid)

	// Prepare unit thumbnail
	unit, err := mdbmodels.ContentUnits(qm.Where("uid=?", uid)).One(common.DB)
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
		ct != consts.CT_RESEARCH_MATERIAL &&
		ct != consts.CT_ARTICLE

	if createThumbnail {
		log.Infof("thumbnail %s [%s]", unit.UID, ct)
		AssetsAPI("thumbnail", unit.UID)
	}
}

func ContentUnitDerivativesChange(e Event) {
	putToIndexer(indexer.ContentUnitUpdate, e.Payload["uid"].(string))
}

func ContentUnitSourcesChange(e Event) {
	putToIndexer(indexer.ContentUnitUpdate, e.Payload["uid"].(string))
}

func ContentUnitTagsChange(e Event) {
	putToIndexer(indexer.ContentUnitUpdate, e.Payload["uid"].(string))
}

func ContentUnitPersonsChange(e Event) {
	putToIndexer(indexer.ContentUnitUpdate, e.Payload["uid"].(string))
}

func ContentUnitPublishersChange(e Event) {
	putToIndexer(indexer.ContentUnitUpdate, e.Payload["uid"].(string))
}

func FilePublished(e Event) {
	putToIndexer(indexer.FileUpdate, e.Payload["uid"].(string))

	uid := e.Payload["uid"].(string)
	file, err := mdbmodels.Files(qm.Where("uid=?", uid)).One(common.DB)
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

func FileReplace(e Event) {
	oFile := e.Payload["old"].(map[string]interface{})
	nFile := e.Payload["new"].(map[string]interface{})

	putToIndexer(indexer.FileUpdate, oFile["uid"].(string))
	putToIndexer(indexer.FileUpdate, nFile["uid"].(string))
}

func FileInsert(e Event) {
	putToIndexer(indexer.FileUpdate, e.Payload["uid"].(string))
}

func FileUpdate(e Event) {
	putToIndexer(indexer.FileUpdate, e.Payload["uid"].(string))

	RemoveFile(e.Payload["uid"].(string))
}

func SourceCreate(e Event) {
	putToIndexer(indexer.SourceUpdate, e.Payload["uid"].(string))
}

func SourceUpdate(e Event) {
	putToIndexer(indexer.SourceUpdate, e.Payload["uid"].(string))
}

func TagCreate(e Event) {
	putToIndexer(indexer.TagUpdate, e.Payload["uid"].(string))
}

func TagUpdate(e Event) {
	putToIndexer(indexer.TagUpdate, e.Payload["uid"].(string))
}

func PersonCreate(e Event) {
	putToIndexer(indexer.PersonUpdate, e.Payload["uid"].(string))
}

func PersonDelete(e Event) {
	putToIndexer(indexer.PersonUpdate, e.Payload["uid"].(string))
}

func PersonUpdate(e Event) {
	putToIndexer(indexer.PersonUpdate, e.Payload["uid"].(string))
}

func PublisherCreate(e Event) {
	putToIndexer(indexer.PublisherUpdate, e.Payload["uid"].(string))
}

func PublisherUpdate(e Event) {
	putToIndexer(indexer.PublisherUpdate, e.Payload["uid"].(string))
}

func BlogPostUpdate(e Event) {
	id := fmt.Sprintf("%d-%d", int64(e.Payload["blogId"].(float64)), int64(e.Payload["wpId"].(float64)))
	putToIndexer(indexer.BlogPostUpdate, id)
}

func BlogPostCreate(e Event) {
	BlogPostUpdate(e)
}

func BlogPostDelete(e Event) {
	BlogPostUpdate(e)
}

func TweetCreate(e Event) {
	putToIndexer(indexer.TweetUpdate, e.Payload["tid"].(string))
}

func TweetUpdate(e Event) {
	putToIndexer(indexer.TweetUpdate, e.Payload["tid"].(string))
}

func TweetDelete(e Event) {
	putToIndexer(indexer.TweetUpdate, e.Payload["tid"].(string))
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
	file, err := mdbmodels.Files(qm.Where("uid=?", uid)).One(common.DB)
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
