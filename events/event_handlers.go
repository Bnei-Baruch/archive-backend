package events

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	log "github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

func putToIndexer(f func(string) error, s string) {
	for {
		select {
		case <-time.After(2 * time.Second):
			fmt.Println("timeout")
		case ChanIndexFuncs <- ChannelForIndexers{
			F: f,
			S: s,
		}:
			return
		}
	}
}

//collection functions
func CollectionCreate(d Data) {
	log.Debugf(d.Payload["uid"].(string))
	putToIndexer(indexer.CollectionAdd, d.Payload["uid"].(string))
}

func CollectionDelete(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.CollectionDelete, d.Payload["uid"].(string))
}

func CollectionUpdate(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.CollectionUpdate, d.Payload["uid"].(string))
}

func CollectionPublishedChange(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.CollectionUpdate, d.Payload["uid"].(string))
}

func CollectionContentUnitsChange(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.CollectionUpdate, d.Payload["uid"].(string))
}

//event functions
func ContentUnitCreate(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.ContentUnitAdd, d.Payload["uid"].(string))
}

func ContentUnitDelete(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.ContentUnitDelete, d.Payload["uid"].(string))
}

func ContentUnitUpdate(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func ContentUnitPublishedChange(d Data) {
	log.Debugf("%+v\n", d)
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))

	unit := GetUnitObj(d.Payload["uid"].(string))
	// check if type needs thumbnail
	NeedThumbnail := true
	unitTypeName := mdb.CONTENT_TYPE_REGISTRY.ByID[unit.TypeID].Name
	excludeUnitTypeNames := []string{"KITEI_MAKOR",
		"LELO_MIKUD", "PUBLICATION", "ARTICLE"}
	for _, b := range excludeUnitTypeNames {
		if b == unitTypeName {
			NeedThumbnail = false
		}
	}
	if NeedThumbnail {
		log.Debugf("Unit %s needs thumbnail because it is of type  \"%s\"", unit.UID, unitTypeName)
	} else {
		log.Debugf("Unit %s needs NO thumbnail because it is of type  \"%s\"", unit.UID, unitTypeName)
	}
	//

	if unit.Published == true &&
		unit.Secure == 0 &&
		NeedThumbnail == true {
		ApiGet(unit.UID, "thumbnail")
	}
}

func ContentUnitDerivativesChange(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func ContentUnitSourcesChange(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func ContentUnitTagsChange(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func ContentUnitPersonsChange(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func ContentUnitPublishersChange(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.ContentUnitUpdate, d.Payload["uid"].(string))
}

func FilePublished(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.FileAdd, d.Payload["uid"].(string))

	fileUid := d.Payload["uid"].(string)
	file := GetFileObj(fileUid)
	if file.Secure != 1 {
		switch file.Type {
		case "image":
			if strings.HasSuffix(file.Name, ".zip") {
				log.Debugf("file %s is zipped image, sending unsip get request to backend", file.UID)
				ApiGet(file.UID, "unzip")
			}

		case "text":
			if file.MimeType.String == "application/msword" {
				log.Debugf("file %s is doc/docx, sending doc2html get request to backend", file.UID)
				ApiGet(file.UID, "doc2html")
			}
		}

	}

}

func FileReplace(d Data) {
	log.Debugf("%+v", d)
	OldUID := d.Payload["old"].(map[string]interface{})
	NewUID := d.Payload["new"].(map[string]interface{})

	putToIndexer(indexer.FileDelete, OldUID["uid"].(string))
	putToIndexer(indexer.FileAdd, NewUID["uid"].(string))
}

func FileInsert(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.FileAdd, d.Payload["uid"].(string))
}

func FileUpdate(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.FileUpdate, d.Payload["uid"].(string))

	removeFile(d.Payload["uid"].(string))

}

func SourceCreate(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.SourceAdd, d.Payload["uid"].(string))
}

func SourceUpdate(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.SourceUpdate, d.Payload["uid"].(string))
}

func TagCreate(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.TagAdd, d.Payload["uid"].(string))
}

func TagUpdate(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.TagUpdate, d.Payload["uid"].(string))
}

func PersonCreate(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.PersonAdd, d.Payload["uid"].(string))
}

func PersonDelete(d Data) {
	log.Debugf("%+v", d)
	//putToIndexer(indexer.PersonDelete, d.Payload["uid"].(string))
}

func PersonUpdate(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.PersonUpdate, d.Payload["uid"].(string))
}

func PublisherCreate(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.PublisherAdd, d.Payload["uid"].(string))
}

func PublisherUpdate(d Data) {
	log.Debugf("%+v", d)
	putToIndexer(indexer.PublisherUpdate, d.Payload["uid"].(string))
}

// GetFileObj gets the file object from db
func GetFileObj(uid string) *mdbmodels.File {
	mdbObj := mdbmodels.FilesG(qm.Where("uid=?", uid))
	OneFile, err := mdbObj.One()
	if err != nil {
		log.Error(err)
	}
	return OneFile
}

// GetUnitObj gets the Unit object from db
func GetUnitObj(uid string) *mdbmodels.ContentUnit {
	mdbObj := mdbmodels.ContentUnitsG(qm.Where("uid=?", uid))
	OneObj, err := mdbObj.One()
	if err != nil {
		log.Error(err)
	}
	return OneObj
}


// UnZipFIle sends request to unzip api by file UID
func ApiGet(uid string, apiType string) error {

	apiURL := viper.GetString("api.url")
	log.Debugf("request url is [%s]", apiURL)
	resp, err := http.Get(apiURL + "/" + apiType + "/" + uid)
	//defer resp.Body.Close()
	if err != nil {
		log.Errorf("%s failed: %+v", apiType, err)
	}
	if resp.StatusCode != 200 {
		log.Errorf("we got response %d for api %s request. UID is \"%s\"", resp.StatusCode, apiType, uid)
	}
	log.Infof("response status code for api call %s. uid \"%s\" is: %d",apiType, uid, resp.StatusCode)
    log.Debugf("the resp status code is %+d",resp.StatusCode)
	return nil
}


//removeFile to send post req to file-api and remove file from search?
func removeFile(s string) error {

	file := GetFileObj(s)
	if file.Secure == 1 &&
		file.Published == true {
		log.Debugf("file %s became secured , sending POST request to api do disable", file.UID)

		type FileBackendRequest struct {
			SHA1     string `json:"sha1"`
			Name     string `json:"name"`
			ClientIP string `json:"clientip,omitempty"`
		}

		data := FileBackendRequest{
			SHA1: hex.EncodeToString(file.Sha1.Bytes),
			Name: file.Name,
		}

		b := new(bytes.Buffer)
		err := json.NewEncoder(b).Encode(data)
		if err != nil {
			return err
		}

		apiURL := viper.GetString("api.url1") + "/api/v1/getremove"
		resp, err := http.Post(apiURL, "application/json; charset=utf-8", b)
		if err != nil {
			log.Errorf("post request to file api failed with %+v", err)
		}
		if resp.StatusCode != 200 {
			log.Errorf("post request to file file api %s  returned status code %d instead of 200", resp.StatusCode)
		}
	}
	return nil
}
