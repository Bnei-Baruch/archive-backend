package events

import (
	"fmt"
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
	log.Info(d.Payload["uid"].(string))

	err := indexer.CollectionAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add collection in ES", err)
	}
}

func CollectionDelete(d Data) {
	log.Infof("%+v", d)

	err := indexer.CollectionDelete(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add collection in ES", err)
	}
}

func CollectionUpdate(d Data) {
	log.Infof("%+v", d)

	err := indexer.CollectionUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update collection in  ES", err)
	}
}

func CollectionPublishedChange(d Data) {
	log.Infof("%+v", d)

	err := indexer.CollectionUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update collection in  ES", err)
	}
}

func CollectionContentUnitsChange(d Data) {
	log.Infof("%+v", d)

	err := indexer.CollectionUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update collection in  ES", err)
	}
}

//event functions
func ContentUnitCreate(d Data) {
	log.Infof("%+v", d)

	err := indexer.ContentUnitAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add content unit to ES", err)
	}
}

func ContentUnitDelete(d Data) {
	log.Infof("%+v", d)

	err := indexer.ContentUnitDelete(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't delete content unit in ES", err)
	}
}

func ContentUnitUpdate(d Data) {
	log.Infof("%+v", d)

	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitPublishedChange(d Data) {
	log.Infof("%+v\n", d)

	unit := GetUnitObj(d.Payload["uid"].(string))

	// check if type needs thumbnail
	NeedThumbnail := true
	unitTypeName := mdb.CONTENT_TYPE_REGISTRY.ByID[unit.TypeID].Name
	thumbnailExcludeTypeNames := []string{"KITEI_MAKOR",
		"LELO_MIKUD", "PUBLICATION", "ARTICLE"}
	for _, b := range thumbnailExcludeTypeNames {
		if b == unitTypeName {
			NeedThumbnail = false
			log.Printf("\nNeedThumbnail: %t because type is  %s\n", NeedThumbnail, unitTypeName)
		}
	}
	//

	if unit.Published == true &&
		unit.Secure == 0 &&
		NeedThumbnail == true {
		apiUrl := viper.GetString("api.url")
		resp, err := http.Get(apiUrl + "/thumbnail/" + unit.UID)
		fmt.Printf("the unit %s status is: %s\n", unit.UID, resp.Status)
		if err != nil {
			log.Errorf("problem sending post request to thumbnail api %V\n", err)
		}
		if resp.StatusCode != 200 {
			log.Errorf("***creating thumbnail for unit %s returned %s instead of 200", unit.UID, resp.Status)
		}

		fmt.Printf("response from post request is: %+v", resp)
	}
	// elastic indexer
	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitDerivativesChange(d Data) {
	log.Infof("%+v", d)

	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitSourcesChange(d Data) {
	log.Infof("%+v", d)

	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitTagsChange(d Data) {
	log.Infof("%+v", d)

	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitPersonsChange(d Data) {
	log.Infof("%+v", d)

	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitPublishersChange(d Data) {
	log.Infof("%+v", d)

	err := indexer.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func FilePublished(d Data) {
	fileUid := d.Payload["uid"].(string)
	log.Infof("%+v", d)

	err := indexer.FileAdd(fileUid)
	if err != nil {
		log.Errorf("couldn't add file to ES", err)
	}

	file := GetFileObj(fileUid)
	if file.Secure != 1 {
		switch file.Type {
		case "image":
			if strings.HasSuffix(file.Name, ".zip") {
				fmt.Printf("************* file is zipped IMAGE:\n %+v", file)
				apiUrl := viper.GetString("api.url")
				resp, err := http.Get(apiUrl + "/unzip/" + file.UID)
				if err != nil {
					log.Errorf("unzip failed: %+v", err)
				}
				fmt.Println(resp)
			}
			if file.MimeType.String == "application/msword" {
				fmt.Printf("************* file is word doc:\n %+v", file)
			}

		case "text":
			if file.MimeType.String == "application/msword" {
				apiUrl := viper.GetString("api.url")
				resp, err := http.Get(apiUrl + "/doc2html/" + file.UID)
				if err != nil {
					log.Errorf("convert doc2html failed: %+v\n", err)
				}
				fmt.Printf("doc2html response: %+v\n", resp)
			}
		}

	}

}

func FileReplace(d Data) {
	log.Infof("%+v", d)
	OldUID := d.Payload["old"].(map[string]interface{})
	NewUID := d.Payload["new"].(map[string]interface{})
	fmt.Printf("\nOLD_ID IS: %v\n", OldUID["uid"])
	fmt.Printf("\nNEW_ID IS: %v\n", NewUID["uid"])

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
	log.Infof("%+v", d)
	err := indexer.FileAdd(d.Payload["uid"].(string))

	if err != nil {
		log.Errorf("couldn't add file to ES", err)
	}
}

func FileUpdate(d Data) {
	log.Infof("%+v", d)

	err := indexer.FileUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update file in ES", err)
	}
}

func SourceCreate(d Data) {
	log.Infof("%+v", d)

	err := indexer.SourceAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't create source in ES", err)
	}
}

func SourceUpdate(d Data) {
	log.Infof("%+v", d)

	err := indexer.SourceUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update source in ES", err)
	}
}

func TagCreate(d Data) {
	log.Infof("%+v", d)

	err := indexer.TagAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add tag in ES", err)
	}
}

func TagUpdate(d Data) {
	log.Infof("%+v", d)

	err := indexer.TagUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update tag in ES", err)
	}
}

func PersonCreate(d Data) {
	log.Infof("%+v", d)

	err := indexer.PersonAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add person in ES", err)
	}
}

func PersonDelete(d Data) {
	log.Infof("%+v", d)

	//err := indexer.PersonDelete(d.Payload["uid"].(string))
	//if err != nil {
	//	log.Errorf("couldn't delete person in ES", err)
	//}
}

func PersonUpdate(d Data) {
	log.Infof("%+v", d)

	err := indexer.PersonUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update person in ES", err)
	}
}

func PublisherCreate(d Data) {
	log.Infof("%+v", d)

	err := indexer.PublisherAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't create publisher in ES", err)
	}
}

func PublisherUpdate(d Data) {
	log.Infof("%+v", d)

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

//err = unZipFile(d.Payload["uid"].(string))
//if err != nil {
//log.Errorf("problem unzipping file %v", d.Payload["uid"].(string), err)
