package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var (
	sofficeBin      string
	docFolder       string
	parseDocsBin    string
	cdnUrl          string
    pythonPath      string
    operatingSystem string
)

func InitVars() {
	pythonPath = viper.GetString("elasticsearch.python-path")
	operatingSystem = viper.GetString("elasticsearch.os")
	sofficeBin = viper.GetString("elasticsearch.soffice-bin")
	if sofficeBin == "" {
		panic("Soffice binary should be set in config.")
	}
	if _, err := os.Stat(sofficeBin); os.IsNotExist(err) {
		panic("Soffice binary not found.")
	}
	parseDocsBin = viper.GetString("elasticsearch.parse-docs-bin")
	if parseDocsBin == "" {
		panic("parse_docs.py binary should be set in config.")
	}
	if _, err := os.Stat(parseDocsBin); os.IsNotExist(err) {
		panic("parse_docs.py not found.")
	}
	docFolder = viper.GetString("elasticsearch.docx-folder")
	utils.Must(os.MkdirAll(docFolder, 0777))
	cdnUrl = viper.GetString("elasticsearch.cdn-url")
	if cdnUrl == "" {
		panic("cdn url should be set in config.")
	}
}

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

func contentUnitsScopeByFile(mdb *sql.DB, fileUID string) ([]string, error) {
	units, err := mdbmodels.ContentUnits(mdb,
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

func CollectionsScopeByFile(mdb *sql.DB, fileUID string) ([]string, error) {
	collections, err := mdbmodels.Collections(mdb,
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

func contentUnitsScopeByCollection(mdb *sql.DB, cUID string) ([]string, error) {
	units, err := mdbmodels.ContentUnits(mdb,
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

func CollectionsScopeByContentUnit(mdb *sql.DB, cuUID string) ([]string, error) {
	collections, err := mdbmodels.Collections(mdb,
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

func contentUnitsScopeBySource(mdb *sql.DB, sourceUID string) ([]string, error) {
	sources, err := mdbmodels.ContentUnits(mdb,
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

func DumpDB(mdb *sql.DB, title string) error {
	fmt.Printf("\n\n ------------------- %s ------------------- \n\n", title)
	units, err := mdbmodels.ContentUnits(mdb).All()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nCONTENT_UNITS\n-------------\n\n")
	for i, unit := range units {
		fmt.Printf("%d: %+v\n", i, unit)
	}

	i18ns, err := mdbmodels.ContentUnitI18ns(mdb).All()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nCONTENT_UNIT_I18N\n-------------\n\n")
	for i, i18n := range i18ns {
		fmt.Printf("%d: %+v\n", i, i18n)
	}

	collections, err := mdbmodels.Collections(mdb).All()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nCOLLECTIONS\n-----------\n\n")
	for i, c := range collections {
		fmt.Printf("%d: %+v\n", i, c)
	}

	ccus, err := mdbmodels.CollectionsContentUnits(mdb).All()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nCOLLECTIONS_CONTENT_UNITS\n-----------\n\n")
	for i, ccu := range ccus {
		fmt.Printf("%d: %+v\n", i, ccu)
	}

	files, err := mdbmodels.Files(mdb).All()
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

func DumpIndexes(esc *elastic.Client, title string) error {
	fmt.Printf("\n\n ------------------- %s ------------------- \n\n", title)
	indexName := IndexName("test", consts.ES_UNITS_INDEX, consts.LANG_ENGLISH)
	fmt.Printf("\n\n\nINDEX %s\n\n", indexName)
	// No need here to specify mdb, docFolder and parseDocsBin.
	indexer := MakeIndexer("test", []string{consts.ES_UNITS_INDEX}, nil, esc)
	if err := indexer.RefreshAll(); err != nil {
		return err
	}
	res, err := esc.Search().Index(indexName).Do(context.TODO())
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
