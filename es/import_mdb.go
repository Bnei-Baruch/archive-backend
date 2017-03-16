package es

import (
	"context"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/vattle/sqlboiler/boil"
	"github.com/vattle/sqlboiler/queries/qm"
	elastic "gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/mdb2es/mdb"
	"github.com/Bnei-Baruch/mdb2es/mdb/models"
	"github.com/Bnei-Baruch/mdb2es/utils"
)

const INDEX_NAME = "mdb_collections"

var (
	db  *sql.DB
	esc *elastic.Client
)

func ImportMDB() {
	var err error
	clock := time.Now()

	// Setup logging
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.Infoln("Import MDB to elasticsearch started")

	log.Info("Setting up connection to MDB")
	db, err = sql.Open("postgres", viper.GetString("mdb.url"))
	utils.Must(err)
	defer db.Close()
	utils.Must(db.Ping())
	boil.SetDB(db)
	//boil.DebugMode = true

	log.Info("Setting up connection to ElasticSearch")
	url := viper.GetString("elasticsearch.url")
	esc, err = elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10*time.Second),
		elastic.SetErrorLog(log.StandardLogger()),
		//elastic.SetInfoLog(log.StandardLogger()),
	)
	utils.Must(err)

	esversion, err := esc.ElasticsearchVersion(url)
	utils.Must(err)
	log.Infof("Elasticsearch version %s", esversion)

	log.Info("Initializing static data from MDB")
	utils.Must(mdb.CONTENT_TYPE_REGISTRY.Init(db))

	utils.Must(recreateIndex(INDEX_NAME))

	collectionsCount := mdbmodels.Collections(db).CountP()
	log.Infof("%d collections in MDB", collectionsCount)

	collections := mdbmodels.Collections(db).AllP()
	//collections := mdbmodels.Collections(db, qm.OrderBy("created_at desc"), qm.Limit(500)).AllP()

	jobs := make(chan *mdbmodels.Collection, 100)
	results := make(chan ProcessResult, 100)

	var workersWG sync.WaitGroup
	for w := 1; w <= 5; w++ {
		workersWG.Add(1)
		go worker(jobs, results, &workersWG)
	}

	var indexersWG sync.WaitGroup
	for w := 1; w <= 5; w++ {
		indexersWG.Add(1)
		go indexer(results, &indexersWG)
	}

	for _, collection := range collections {
		jobs <- collection
	}

	log.Info("Closing jobs channel")
	close(jobs)

	log.Info("Waiting for workers to finish")
	workersWG.Wait()

	log.Info("Closing results channel")
	close(results)

	log.Info("Waiting for indexers to finish")
	indexersWG.Wait()

	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())

}

type ProcessResult struct {
	mdb *mdbmodels.Collection
	es  *Collection
	err error
}

func worker(jobs <-chan *mdbmodels.Collection, results chan<- ProcessResult, wg *sync.WaitGroup) {
	for j := range jobs {
		c, err := processCollection(j)
		results <- ProcessResult{mdb: j, es: c, err: err}
	}
	wg.Done()
}

func indexer(jobs <-chan ProcessResult, wg *sync.WaitGroup) {
	for j := range jobs {
		if j.err == nil {
			log.Infof("Indexing collection %d", j.mdb.ID)
			_, err := esc.Index().
				Index(INDEX_NAME).
				Type("collection").
				Id(j.mdb.UID).
				BodyJson(j.es).
				Do(context.TODO())
			if err != nil {
				log.Errorf("Error indexing, collection_id [%d]: %s", j.mdb.ID, err)
			}
		} else {
			if strings.HasPrefix(j.err.Error(), "Empty collection") {
				log.Warn(j.err.Error())
			} else {
				log.Errorf("Error processing collection: ", j.mdb.ID, j.err)
			}
		}
	}
	wg.Done()
}

func processCollection(collection *mdbmodels.Collection) (*Collection, error) {
	log.Infof("Processing collection %d", collection.ID)

	if !collection.Properties.Valid {
		return nil, errors.Errorf("Invalid properties, collection_id [%d]", collection.ID)
	}

	// Fetch and validate related data
	err := collection.L.LoadCollectionsContentUnits(db, true, collection)
	if err != nil {
		return nil, errors.Wrapf(err, "Error loading related units, collection_id [%d]", collection.ID)
	}
	if len(collection.R.CollectionsContentUnits) == 0 {
		return nil, errors.Errorf("Empty collection [%d]", collection.ID)
	}

	err = collection.L.LoadCollectionI18ns(db, true, collection)
	if err != nil {
		return nil, errors.Wrapf(err, "Error loading i18n, collection_id [%d]", collection.ID)
	}

	var props mdb.CollectionProperties
	err = collection.Properties.Unmarshal(&props)
	if err != nil {
		return nil, errors.Wrapf(err, "Error unmarshaling properties, collection_id [%d]", collection.ID)
	}

	c := &Collection{
		MDB_UID:     collection.UID,
		ContentType: mdb.CONTENT_TYPE_REGISTRY.ByID[collection.TypeID].Name,
		FilmDate:    props.FilmDate.Time,
	}

	// i18n
	c.Names = make(map[string]string, len(collection.R.CollectionI18ns))
	c.Descriptions = make(map[string]string, len(collection.R.CollectionI18ns))
	for _, x := range collection.R.CollectionI18ns {
		if x.Name.Valid {
			c.Names[x.Language] = x.Name.String
		}
		if x.Description.Valid {
			c.Descriptions[x.Language] = x.Description.String
		}
	}

	// Content Units
	c.ContentUnits = make([]*ContentUnit, 0)
	for _, ccu := range collection.R.CollectionsContentUnits {
		u, err := processContentUnit(ccu.ContentUnitID)
		if err == nil {
			u.NameInCollection = ccu.Name
			c.ContentUnits = append(c.ContentUnits, u)
		} else {
			if strings.HasPrefix(err.Error(), "Empty content unit") {
				log.Warn(err.Error())
			} else {
				return nil, errors.Wrapf(err, "Error processing content unit [%d]", ccu.ContentUnitID)
			}
		}
	}

	if len(c.ContentUnits) == 0 {
		return nil, errors.Errorf("Empty collection [%d]", collection.ID)
	}

	sort.Sort(ByNameInCollection{c.ContentUnits})

	return c, nil
}

func processContentUnit(cuID int64) (*ContentUnit, error) {
	log.Debugf("Processing content unit %d", cuID)

	// Fetch and validate related data
	unit, err := mdbmodels.ContentUnits(db,
		qm.Where("id = ?", cuID),
		qm.Load("ContentUnitI18ns")).
		One()
	if err != nil {
		return nil, errors.Wrapf(err, "Error loading content unit [%d]", cuID)
	}
	if !unit.Properties.Valid {
		return nil, errors.Errorf("Invalid properties, content_unit_id [%d]", cuID)
	}

	files, err := mdbmodels.Files(db,
		qm.Where("content_unit_id = ?", cuID),
		qm.And("properties ->> 'url' is not null")).
		All()
	if err != nil {
		return nil, errors.Wrapf(err, "Error loading uploaded files, content_unit_id [%d]", cuID)
	}
	if len(files) == 0 {
		return nil, errors.Errorf("Empty content unit [%d]", cuID)
	}

	var props mdb.ContentUnitProperties
	err = unit.Properties.Unmarshal(&props)
	if err != nil {
		return nil, errors.Wrapf(err, "Error unmarshaling properties, content_unit_id [%d]", cuID)
	}

	u := &ContentUnit{
		MDB_UID:          unit.UID,
		ContentType:      mdb.CONTENT_TYPE_REGISTRY.ByID[unit.TypeID].Name,
		FilmDate:         props.FilmDate.Time,
		Secure:           props.Secure,
		Duration:         props.Duration,
		OriginalLanguage: props.OriginalLanguage,
	}

	// i18n
	u.Names = make(map[string]string, len(unit.R.ContentUnitI18ns))
	u.Descriptions = make(map[string]string, len(unit.R.ContentUnitI18ns))
	for _, x := range unit.R.ContentUnitI18ns {
		if x.Name.Valid {
			u.Names[x.Language] = x.Name.String
		}
		if x.Description.Valid {
			u.Descriptions[x.Language] = x.Description.String
		}
	}

	// Files
	u.Files = make([]*File, len(files))
	for k, file := range files {
		var f *File
		f, err = processFile(file)
		if err == nil {
			u.Files[k] = f
		} else {
			return nil, errors.Wrapf(err, "Error processing file [%d]", file.ID)
		}
	}

	return u, nil
}

func processFile(file *mdbmodels.File) (*File, error) {
	log.Debugf("Processing file %d", file.ID)

	var props mdb.FileProperties
	err := file.Properties.Unmarshal(&props)
	if err != nil {
		return nil, errors.Wrapf(err, "Error loading properties, file_id [%d]", file.ID)
	}

	f := &File{
		MDB_UID:  file.UID,
		Name:     file.Name,
		Size:     file.Size,
		Type:     file.Type,
		SubType:  file.SubType,
		URL:      props.URL,
		Secure:   props.Secure,
		Duration: props.Duration,
	}

	if file.FileCreatedAt.Valid {
		f.FilmDate = file.FileCreatedAt.Time
	}
	if file.Language.Valid {
		f.Language = file.Language.String
	}
	if file.MimeType.Valid {
		f.MimeType = file.MimeType.String
	}

	return f, nil
}

func recreateIndex(name string) error {
	ctx := context.TODO()

	exists, err := esc.IndexExists(name).Do(ctx)
	if exists {
		log.Infof("Index %s already exist, deleting...", name)
		_, err = esc.DeleteIndex(name).Do(ctx)
		if err != nil {
			return err
		}
	}

	log.Infof("Creating index: %s", name)
	mappings, err := ioutil.ReadFile("es/mappings.json")
	if err != nil {
		log.Fatal(err)
	}
	var bodyJson map[string]interface{}
	if err = json.Unmarshal(mappings, &bodyJson); err != nil {
		log.Fatal(err)
	}
	_, err = esc.CreateIndex(name).BodyJson(bodyJson).Do(ctx)
	//_, err = esc.CreateIndex(name).Do(ctx)
	if err != nil {
		return err
	}

	// Settings for bulk indexing.
	// TODO: These should be reverted back when done
	_, err = esc.IndexPutSettings(name).BodyJson(map[string]interface{}{
		"refresh_interval":   "-1",
		"number_of_replicas": 0,
	}).Do(ctx)

	return err
}
