package es

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/volatiletech/sqlboiler/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

func uidToTypedUID(t string, uid string) string {
	return fmt.Sprintf("%s:%s", t, uid)
}

func uidsToTypedUIDs(t string, uids []string) []string {
	ret := make([]string, len(uids))
	for i, uid := range uids {
		ret[i] = uidToTypedUID(t, uid)
	}
	return ret
}

// Scopes - for detection of changes

func contentUnitsScopeByFile(fileUID string) ([]string, error) {
	units, err := mdbmodels.ContentUnits(mdb.DB,
		qm.InnerJoin("files AS f on f.content_unit_id = content_units.id"),
		qm.Where("f.uid = ?", fileUID)).All()
	if err != nil {
		return nil, err
	}
	uids := make([]string, len(units))
	for i, unit := range units {
		uids[i] = unit.UID
	}
	return uids, nil
}

func collectionsScopeByFile(fileUID string) ([]string, error) {
	collections, err := mdbmodels.Collections(mdb.DB,
		qm.InnerJoin("collections_content_units AS ccu ON ccu.collection_id = collections.id"),
		qm.InnerJoin("content_units AS cu ON ccu.content_unit_id = cu.id"),
		qm.InnerJoin("files AS f on f.content_unit_id = cu.id"),
		qm.Where("f.uid = ?", fileUID)).All()
	if err != nil {
		return nil, err
	}
	uids := make([]string, len(collections))
	for i, collection := range collections {
		uids[i] = collection.UID
	}
	return uids, nil
}

func contentUnitsScopeByCollection(cUID string) ([]string, error) {
	units, err := mdbmodels.ContentUnits(mdb.DB,
		qm.InnerJoin("collections_content_units AS ccu ON ccu.content_unit_id = content_units.id"),
		qm.InnerJoin("collections AS c ON ccu.collection_id = c.id"),
		qm.Where("c.uid = ?", cUID)).All()
	if err != nil {
		return nil, err
	}
	uids := make([]string, len(units))
	for i, unit := range units {
		uids[i] = unit.UID
	}
	return uids, nil
}

func collectionsScopeByContentUnit(cuUID string) ([]string, error) {
	collections, err := mdbmodels.Collections(mdb.DB,
		qm.InnerJoin("collections_content_units AS ccu ON ccu.collection_id = collections.id"),
		qm.InnerJoin("content_units AS cu ON ccu.content_unit_id = cu.id"),
		qm.Where("cu.uid = ?", cuUID)).All()
	if err != nil {
		return nil, err
	}
	uids := make([]string, len(collections))
	for i, collection := range collections {
		uids[i] = collection.UID
	}
	return uids, nil
}

func contentUnitsScopeBySource(sourceUID string) ([]string, error) {
	sources, err := mdbmodels.ContentUnits(mdb.DB,
		qm.InnerJoin("content_units_sources AS cus ON cus.content_unit_id = id"),
		qm.InnerJoin("sources AS s ON s.id = cus.source_id"),
		qm.Where("s.uid = ?", sourceUID)).All()
	if err != nil {
		return nil, err
	}
	uids := make([]string, len(sources))
	for i, sources := range sources {
		uids[i] = sources.UID
	}
	return uids, nil
}

// DEBUG FUNCTIONS

func dumpDB(title string) error {
	fmt.Printf("\n\n ------------------- %s ------------------- \n\n", title)
	units, err := mdbmodels.ContentUnits(mdb.DB).All()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nCONTENT_UNITS\n-------------\n\n")
	for i, unit := range units {
		fmt.Printf("%d: %+v\n", i, unit)
	}

	i18ns, err := mdbmodels.ContentUnitI18ns(mdb.DB).All()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nCONTENT_UNIT_I18N\n-------------\n\n")
	for i, i18n := range i18ns {
		fmt.Printf("%d: %+v\n", i, i18n)
	}

	collections, err := mdbmodels.Collections(mdb.DB).All()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nCOLLECTIONS\n-----------\n\n")
	for i, c := range collections {
		fmt.Printf("%d: %+v\n", i, c)
	}

	ccus, err := mdbmodels.CollectionsContentUnits(mdb.DB).All()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nCOLLECTIONS_CONTENT_UNITS\n-----------\n\n")
	for i, ccu := range ccus {
		fmt.Printf("%d: %+v\n", i, ccu)
	}

	files, err := mdbmodels.Files(mdb.DB).All()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nFILES\n-------------\n\n")
	for i, file := range files {
		fmt.Printf("%d: %+v\n", i, file)
	}

	fmt.Printf("\n\n ------------------- END OF %s ------------------- \n\n", title)
	return nil
}

func dumpIndexes(title string) error {
	fmt.Printf("\n\n ------------------- %s ------------------- \n\n", title)
	indexName := IndexName("test", consts.ES_UNITS_INDEX, consts.LANG_ENGLISH)
	fmt.Printf("\n\n\nINDEX %s\n\n", indexName)
	indexer := MakeIndexer("test", []string{consts.ES_UNITS_INDEX})
	if err := indexer.RefreshAll(); err != nil {
		return err
	}
	res, err := mdb.ESC.Search().Index(indexName).Do(context.TODO())
	if err != nil {
		return err
	}
	for i, hit := range res.Hits.Hits {
		var cu ContentUnit
		json.Unmarshal(*hit.Source, &cu)
		fmt.Printf("%d: %+v\n", i, cu)
	}
	fmt.Printf("\n\n ------------------- END OF %s ------------------- \n\n", title)
	return err
}
