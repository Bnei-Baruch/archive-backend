package events

import (
	"github.com/Bnei-Baruch/archive-backend/es"
	log "github.com/Sirupsen/logrus"
)

//collection functions
func CollectionCreate(d Data) {
	log.Info(d.Payload["uid"].(string))

	err := es.CollectionAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add collection in ES", err)
	}
}

func CollectionDelete(d Data) {
	log.Infof("%+v", d)

	err := es.CollectionDelete(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add collection in ES", err)
	}
}

func CollectionUpdate(d Data) {
	log.Infof("%+v", d)

	err := es.CollectionUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update collection in  ES", err)
	}
}

//
func CollectionPublishedChange(d Data) {
	log.Infof("%+v", d)

	err := es.CollectionUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update collection in  ES", err)
	}
}

func CollectionContentUnitsChange(d Data) {
	log.Infof("%+v", d)

	err := es.CollectionUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update collection in  ES", err)
	}
}

//
////event functions
func ContentUnitCreate(d Data) {
	log.Infof("%+v", d)

	err := es.ContentUnitAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add content unit to ES", err)
	}
}

func ContentUnitDelete(d Data) {
	log.Infof("%+v", d)

	err := es.ContentUnitDelete(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't delete content unit in ES", err)
	}
}

func ContentUnitUpdate(d Data) {
	log.Infof("%+v", d)

	err := es.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitPublishedChange(d Data) {
	log.Infof("%+v", d)

	err := es.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitDerivativesChange(d Data) {
	log.Infof("%+v", d)

	err := es.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitSourcesChange(d Data) {
	log.Infof("%+v", d)

	err := es.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitTagsChange(d Data) {
	log.Infof("%+v", d)

	err := es.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitPersonsChange(d Data) {
	log.Infof("%+v", d)

	err := es.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func ContentUnitPublishersChange(d Data) {
	log.Infof("%+v", d)

	err := es.ContentUnitUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
}

func FilePublished(d Data) {
	log.Infof("%+v", d)

	err := es.FileAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add file to ES", err)
	}

	err = unZipFile(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("problem unzipping file %v", d.Payload["uid"].(string), err)
	}
}

func FileReplace(d Data) {
	log.Infof("%+v", d)
	//errReplace := es.FileDelete(oldUid)
	//if errReplace != nil {
	//	log.Errorf("couldn't delete file from ES", errReplace)
	//}
	//
	//errAdd := es.FileAdd(d.Payload["uid"].(string))
	//if errAdd != nil {
	//	log.Errorf("couldn't add file to ES", errAdd)
	//}

}

func FileInsert(d Data) {
	log.Infof("%+v", d)
	err := es.FileAdd(d.Payload["uid"].(string))

	if err != nil {
		log.Errorf("couldn't add file to ES", err)
	}
}

func FileUpdate(d Data) {
	log.Infof("%+v", d)

	err := es.FileUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update file in ES", err)
	}
}

func SourceCreate(d Data) {
	log.Infof("%+v", d)

	err := es.SourceAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't create source in ES", err)
	}
}

func SourceUpdate(d Data) {
	log.Infof("%+v", d)

	err := es.SourceUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update source in ES", err)
	}
}

func TagCreate(d Data) {
	log.Infof("%+v", d)

	err := es.TagAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add tag in ES", err)
	}
}

func TagUpdate(d Data) {
	log.Infof("%+v", d)

	err := es.TagUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update tag in ES", err)
	}
}

func PersonCreate(d Data) {
	log.Infof("%+v", d)

	err := es.PersonAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't add person in ES", err)
	}
}

func PersonDelete(d Data) {
	log.Infof("%+v", d)

	err := es.PersonDelete(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't delete person in ES", err)
	}
}

func PersonUpdate(d Data) {
	log.Infof("%+v", d)

	err := es.PersonUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update person in ES", err)
	}
}

func PublisherCreate(d Data) {
	log.Infof("%+v", d)

	err := es.PublisherAdd(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't create publisher in ES", err)
	}
}

func PublisherUpdate(d Data) {
	log.Infof("%+v", d)

	err := es.PublisherUpdate(d.Payload["uid"].(string))
	if err != nil {
		log.Errorf("couldn't update publisher in ES", err)
	}
	
}
