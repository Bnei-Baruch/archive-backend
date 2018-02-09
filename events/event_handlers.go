package events

import (
	"net/http"
	"strings"

	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	log "github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/mdb"
)

//collection functions
func CollectionCreate(d Data) {
	log.Debugf(d.Payload["uid"].(string))

	err := indexer.CollectionAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add collection in ES", err)
	}
}

func CollectionDelete(d Data) {
	log.Debugf("%+v", d)

	err := indexer.CollectionDelete(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add collection in ES", err)
	}
}

func CollectionUpdate(d Data) {
	log.Debugf("%+v", d)

	err := indexer.CollectionUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update collection in  ES", err)
	}
}

func CollectionPublishedChange(d Data) {
	log.Debugf("%+v", d)

	err := indexer.CollectionUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update collection in  ES", err)
	}
}

func CollectionContentUnitsChange(d Data) {
	log.Debugf("%+v", d)

	err := indexer.CollectionUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update collection in  ES", err)
	}
}

//event functions
func ContentUnitCreate(d Data) {
	log.Debugf("%+v", d)

	err := indexer.ContentUnitAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add content unit to ES", err)
	}
}

func ContentUnitDelete(d Data) {
	log.Debugf("%+v", d)

	err := indexer.ContentUnitDelete(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't delete content unit in ES", err)
	}
}

func ContentUnitUpdate(d Data) {
	log.Debugf("%+v", d)

	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitPublishedChange(d Data) {
	log.Debugf("%+v\n", d)

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
		log.Debugf("Unit %s needs thumbnail because it is of type  \"%s\"", unit.UID,  unitTypeName)
	} else {
		log.Debugf("Unit %s needs NO thumbnail because it is of type  \"%s\"", unit.UID,  unitTypeName)
	}
	//

	if unit.Published == true &&
		unit.Secure == 0 &&
		NeedThumbnail == true {
		apiUrl := viper.GetString("api.url")
		resp, err := http.Get(apiUrl + "/thumbnail/" + unit.UID)
		log.Infof("response status code for creating  unit \"%s\" thumbnail of type \"%s\" is: %d", unit.UID ,unitTypeName, resp.StatusCode)
		if err != nil {
			log.Errorf("problem sending post request to thumbnail api %V\n", err)
		}
		if resp.StatusCode != 200 {
			log.Errorf("creating thumbnail for unit %s returned status code %d instead of 200", unit.UID, resp.StatusCode)
		}
	}
	// elastic indexer
	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitDerivativesChange(d Data) {
	log.Debugf("%+v", d)

	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitSourcesChange(d Data) {
	log.Debugf("%+v", d)

	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitTagsChange(d Data) {
	log.Debugf("%+v", d)

	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitPersonsChange(d Data) {
	log.Debugf("%+v", d)

	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitPublishersChange(d Data) {
	log.Debugf("%+v", d)

	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func FilePublished(d Data) {
	log.Debugf("%+v", d)

	fileUid := d.Payload["uid"].(string)

	err := indexer.FileAdd(fileUid)
	if err != nil {
		log.Errorf("couldn't add file to ES", err)
	}

	file := GetFileObj(fileUid)
	if file.Secure != 1 {
		switch file.Type {
		case "image":
			if strings.HasSuffix(file.Name, ".zip") {
				log.Debugf("file %s is zipped image, sending unsip post request to backend", file.UID)
				apiUrl := viper.GetString("api.url")
				resp, err := http.Get(apiUrl + "/unzip/" + file.UID)
				if err != nil {
					log.Errorf("unzip failed: %+v", err)
				}
				if resp.StatusCode != 200 {
					log.Errorf("we got response %d for api unzip request. file UID is \"%s\"", resp.StatusCode, file.UID)
				}
				log.Infof("response status code for unzipping file \"%s\" is: %d", file.UID, resp.StatusCode)
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
	//fmt.Printf("\nOLD_ID IS: %v\n", OldUID["uid"])
	//fmt.Printf("\nNEW_ID IS: %v\n", NewUID["uid"])

	errReplace := indexer.FileDelete(OldUID["uid"].(string))
	if errReplace != nil {
		log.Errorf("couldn't delete file from ES", errReplace)
	}

	errAdd := indexer.FileAdd(NewUID["uid"].(string))
	if errAdd != nil {
		log.Errorf("couldn't add file to ES", errAdd)
	}

}

func FileInsert(d Data) {
	log.Debugf("%+v", d)
	err := indexer.FileAdd(d.Payload["uid"].(string))

	if err != nil {
		log.Errorf("couldn't add file to ES", err)
	}
}

func FileUpdate(d Data) {
	log.Debugf("%+v", d)

	err := indexer.FileUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update file in ES", err)
	}
}

func SourceCreate(d Data) {
	log.Debugf("%+v", d)

	err := indexer.SourceAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't create source in ES", err)
	}
}

func SourceUpdate(d Data) {
	log.Debugf("%+v", d)

	err := indexer.SourceUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update source in ES", err)
	}
}

func TagCreate(d Data) {
	log.Debugf("%+v", d)

	err := indexer.TagAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add tag in ES", err)
	}
}

func TagUpdate(d Data) {
	log.Debugf("%+v", d)

	err := indexer.TagUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update tag in ES", err)
	}
}

func PersonCreate(d Data) {
	log.Debugf("%+v", d)

	err := indexer.PersonAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add person in ES", err)
	}
}

func PersonDelete(d Data) {
	log.Debugf("%+v", d)

	//err := indexer.PersonDelete(d.Payload["uid"].(string))
	//if err != nil {
	//	log.Errorf("couldn't delete person in ES", err)
	//}
}

func PersonUpdate(d Data) {
	log.Debugf("%+v", d)

	err := indexer.PersonUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update person in ES", err)
	}
}

func PublisherCreate(d Data) {
	log.Debugf("%+v", d)

	err := indexer.PublisherAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't create publisher in ES", err)
	}
}

func PublisherUpdate(d Data) {
	log.Debugf("%+v", d)

	err := indexer.PublisherUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update publisher in ES", err)
	}
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

func GetUnitObj(uid string) *mdbmodels.ContentUnit {
	mdbObj := mdbmodels.ContentUnitsG(qm.Where("uid=?", uid))
	OneObj, err := mdbObj.One()
	if err != nil {
		log.Error(err)
	}

	return OneObj
}

