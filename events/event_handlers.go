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
		apiUrl := viper.GetString("api.url")
		resp, err := http.Get(apiUrl + "/thumbnail/" + unit.UID)
		log.Infof("response status code for creating  unit \"%s\" thumbnail of type \"%s\" is: %d", unit.UID, unitTypeName, resp.StatusCode)
		if err != nil {
			log.Errorf("problem sending post request to thumbnail api %V\n", err)
		}
		if resp.StatusCode != 200 {
			log.Errorf("creating thumbnail for unit %s returned status code %d instead of 200", unit.UID, resp.StatusCode)
		}
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
				log.Debugf("file %s is zipped image, sending unsip post request to backend", file.UID)
				err := UnZipFIle(file.UID)
				if err != nil {
					log.Errorf("couldn't unzip %s\n because of error %+v", file.UID, err)
				}
				//apiUrl := viper.GetString("api.url")
				//resp, err := http.Get(apiUrl + "/unzip/" + file.UID)
				//if err != nil {
				//	log.Errorf("unzip failed: %+v", err)
				//}
				//if resp.StatusCode != 200 {
				//	log.Errorf("we got response %d for api unzip request. file UID is \"%s\"", resp.StatusCode, file.UID)
				//}
				//log.Infof("response status code for unzipping file \"%s\" is: %d", file.UID, resp.StatusCode)
			}

		case "text":
			if file.MimeType.String == "application/msword" {
				apiUrl := viper.GetString("api.url")
				resp, err := http.Get(apiUrl + "/doc2html/" + file.UID)
				log.Debugf("doc2html response: %+v\n", resp)
				if err != nil {
					log.Errorf("convert doc2html failed: %+v\n", err)
				}
				if resp.StatusCode != 200 {
					log.Errorf("we got response %d for api post doc2html request. file UID is \"%s\"", resp.StatusCode, file.UID)
				}
				log.Infof("response status code for doc2htmling file \"%s\" is: %d", file.UID, resp.StatusCode)
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
func UnZipFIle(uid string) error {

	apiUrl := viper.GetString("api.url")
	resp, err := http.Get(apiUrl + "/unzip/" + uid)
	if err != nil {
		log.Errorf("unzip failed: %+v", err)
	}
	if resp.StatusCode != 200 {
		log.Errorf("we got response %d for api unzip request. file UID is \"%s\"", resp.StatusCode, uid)
	}
	log.Infof("response status code for unzipping file \"%s\" is: %d", uid, resp.StatusCode)
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

		apiUrl := viper.GetString("api.url1") + "/api/v1/getremove"
		resp, err := http.Post(apiUrl, "application/json; charset=utf-8", b)
		if err != nil {
			log.Errorf("post request to file api failed with %+v", err)
		}
		if resp.StatusCode != 200 {
			log.Errorf("post request to file file api %s  returned status code %d instead of 200", resp.StatusCode)
		}
	}
	return nil
}
