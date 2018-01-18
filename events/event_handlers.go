package events

import (
	"github.com/Bnei-Baruch/archive-backend/es"
	log "github.com/Sirupsen/logrus"
)


//collection functions
func CollectionCreate(uid string) error {
	err := es.CollectionAdd(uid)
	if err != nil {
		log.Errorf("couldn't add collection in ES", err)
	}
	return nil
}

func CollectionDelete(uid string) error {
	err := es.CollectionDelete(uid)
	if err != nil {
		log.Errorf("couldn't add collection in ES", err)
	}
	return nil
}

func CollectionUpdate(uid string) error {
	err := es.CollectionUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update collection in  ES", err)
	}
	return nil
}

func CollectionPublishedChange(uid string) error {
	err := es.CollectionUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update collection in  ES", err)
	}
	return nil
}

func CollectionContentUnitsChange(uid string) error {
	err := es.CollectionUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update collection in  ES", err)
	}
	return nil
}

//event functions
func ContentUnitCreate(uid string) error {
	err := es.ContentUnitAdd(uid)
	if err != nil {
		log.Errorf("couldn't add content unit to ES", err)
	}
	return nil
}

func ContentUnitDelete(uid string) error {
	err := es.ContentUnitDelete(uid)
	if err != nil {
		log.Errorf("couldn't add delete unit to ES", err)
	}
	return nil
}

func ContentUnitUpdate(uid string) error {
	err := es.ContentUnitUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
	return nil
}

func ContentUnitPublishedChange(uid string) error {
	err := es.ContentUnitUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
	return nil
}

func ContentUnitDerivativesChange(uid string) error {
	err := es.ContentUnitUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
	return nil
}

func ContentUnitSourcesChange(uid string) error {
	err := es.ContentUnitUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
	return nil
}

func ContentUnitTagsChange(uid string) error {
	err := es.ContentUnitUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
	return nil
}

func ContentUnitPersonsChange(uid string) error {
	err := es.ContentUnitUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
	return nil
}

func ContentUnitPublishersChange(uid string) error {
	err := es.ContentUnitUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update content unit in ES", err)
	}
	return nil
}

func FilePublished(uid string) error {
	err := es.FileAdd(uid)
	if err != nil {
		log.Errorf("couldn't add file to ES", err)
	}
	return nil
}

func FileReplace(uid string, oldUid string) error {
	errReplace := es.FileDelete(oldUid)
	if errReplace != nil {
		log.Errorf("couldn't delete file from ES", errReplace)
	}

	errAdd := es.FileAdd(uid)
	if errAdd != nil {
		log.Errorf("couldn't add file to ES", errAdd)
	}

	return nil
}

func FileInsert(uid string) error {
	err := es.FileAdd(uid)
	if err != nil {
		log.Errorf("couldn't add file to ES", err)
	}
	return nil
}

func FileUpdate(uid string) error {
	err := es.FileUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update file in ES", err)
	}
	return nil
}

func SourceCreate(uid string) error {
	err := es.SourceAdd(uid)
	if err != nil {
		log.Errorf("couldn't create source in ES", err)
	}
	return nil
}

func SourceUpdate(uid string) error {
	err := es.SourceUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update source in ES", err)
	}
	return nil
}

func TagCreate(uid string) error {
	err := es.TagAdd(uid)
	if err != nil {
		log.Errorf("couldn't add tag in ES", err)
	}
	return nil
}

func TagUpdate(uid string) error {
	err := es.TagUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update tag in ES", err)
	}
	return nil
}

func PersonCreate(uid string) error {
	err := es.PersonAdd(uid)
	if err != nil {
		log.Errorf("couldn't add person in ES", err)
	}
	return nil
}

func PersonDelete(uid string) error {
	err := es.PersonDelete(uid)
	if err != nil {
		log.Errorf("couldn't delete person in ES", err)
	}
	return nil
}

func PersonUpdate(uid string) error {
	err := es.PersonUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update person in ES", err)
	}
	return nil
}

func PublisherCreate(uid string) error {
	err := es.PublisherAdd(uid)
	if err != nil {
		log.Errorf("couldn't create publisher in ES", err)
	}
	return nil
}

func PublisherUpdate(uid string) error {
	err := es.PublisherUpdate(uid)
	if err != nil {
		log.Errorf("couldn't update publisher in ES", err)
	}
	return nil
}










