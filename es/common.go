package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

var (
	sourcesFolder string
	sofficeBin    string
	docFolder     string
	parseDocsBin  string
	cdnUrl        string
	pythonPath    string
)

func DocFolder() (string, error) {
	return InitConfigFolder("elasticsearch.docx-folder", &docFolder)
}

func SourcesFolder() (string, error) {
	return InitConfigFolder("elasticsearch.sources-folder", &sourcesFolder)
}

func InitConfigFolder(configKey string, value *string) (string, error) {
	if *value != "" {
		return *value, nil
	}

	path := viper.GetString(configKey)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(docFolder, 0777)
			if err != nil {
				*value = path
			}
			return path, err
		} else {
			return path, err
		}
	}
	*value = path
	return *value, nil
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func InitVars() {
	pythonPath = viper.GetString("elasticsearch.python-path")
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
	cdnUrl = viper.GetString("elasticsearch.cdn-url")
	if cdnUrl == "" {
		panic("cdn url should be set in config.")
	}
}

func keyValue(t string, uid string) string {
	return fmt.Sprintf("%s:%s", t, uid)
}

func KeyValues(t string, uids []string) []string {
	ret := make([]string, len(uids))
	for i, uid := range uids {
		ret[i] = keyValue(t, uid)
	}
	return ret
}

func KeyIValues(t string, uids []string) []interface{} {
	ret := make([]interface{}, len(uids))
	for i, uid := range uids {
		ret[i] = keyValue(t, uid)
	}
	return ret
}

func KeyValuesToValues(t string, typedUIDs []string) ([]string, error) {
	ret := make([]string, 0)
	for _, typedUid := range typedUIDs {
		parts := strings.Split(typedUid, ":")
		if len(parts) != 2 {
			return []string{}, errors.New(fmt.Sprintf("Bad typed uid %s expected 'type:value'.", typedUIDs))
		}
		if parts[0] == t {
			ret = append(ret, parts[1])
		}
	}
	return ret, nil
}

func (result *Result) ToString() string {
	resultCopy := result
	if len(resultCopy.Content) > 30 {
		resultCopy.Content = fmt.Sprintf("%s...", resultCopy.Content[:30])
	}
	resultBytes, err := json.Marshal(resultCopy)
	if err != nil {
		return "<BAD Result>"
	}
	return string(resultBytes)
}

func Suffixes(title string) []string {
	parts := strings.Split(strings.TrimSpace(title), " ")
	ret := []string{}
	for i, _ := range parts {
		ret = append(ret, strings.Join(parts[i:], " "))
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
	fmt.Printf("\n\n ------------------- %s DUMP DB ------------------- \n\n", title)
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

	ci18ns, err := mdbmodels.CollectionI18ns(mdb).All()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nCOLLECTION_I18N\n-------------\n\n")
	for i, ci18n := range ci18ns {
		fmt.Printf("%d: %+v\n", i, ci18n)
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

	sources, err := mdbmodels.Sources(mdb).All()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nSOURCES\n-------------\n\n")
	for i, source := range sources {
		fmt.Printf("%d: %+v\n", i, source)
	}

	tags, err := mdbmodels.Tags(mdb).All()
	if err != nil {
		return err
	}
	fmt.Printf("\n\nTAGS\n-------------\n\n")
	for i, tag := range tags {
		fmt.Printf("%d: %+v\n", i, tag)
	}

	fmt.Printf("\n\n ------------------- END OF %s DUMP DB ------------------- \n\n", title)
	return nil
}

func DumpIndexes(esc *elastic.Client, title string, resultType string) error {
	fmt.Printf("\n\n ------------------- %s DUMP INDEXES ------------------- \n\n", title)
	indexName := IndexName("test", consts.ES_RESULTS_INDEX, consts.LANG_ENGLISH, "test-date")
	fmt.Printf("\n\n\nINDEX %s\n\n", indexName)
	indexer, err := MakeIndexer("test", "test-date", []string{resultType}, nil, esc)
	if err != nil {
		return err
	}
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
	fmt.Printf("\n\n ------------------- END OF %s DUMP INDEXES ------------------- \n\n", title)
	return err
}
