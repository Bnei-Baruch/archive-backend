package es

import (
    "context"
	"encoding/json"
	"fmt"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
)

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

