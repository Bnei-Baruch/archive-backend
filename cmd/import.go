package cmd

import (
	//"time"
	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"database/sql"
	_ "github.com/lib/pq"
	"github.com/vattle/sqlboiler/boil"
	// Dot import so we can access query mods directly instead of prefixing with "qm."
	. "github.com/vattle/sqlboiler/queries/qm"
	elastic "gopkg.in/olivere/elastic.v5"

	"github.com/Bnei-Baruch/mdb2es/kmedia"
	"time"
	"golang.org/x/net/context"
	"github.com/Bnei-Baruch/mdb2es/models"
	"math"
	"encoding/json"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "ETL from kmedia to ES",
	Run: importFn,
}

func init() {
	RootCmd.AddCommand(importCmd)

}

func importDefaults() {
	viper.SetDefault("kmedia", map[string]interface{}{
		"url": "postgres://localhost/kmedia?sslmode=disable",
	})
	viper.SetDefault("elasticsearch", map[string]interface{}{
		"url": "http://127.0.0.1:9200",
	})
}

func importFn(cmd *cobra.Command, args []string) {
	importDefaults()

	// Setup logging
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.Infoln("Import started")

	// Setup DB
	db, err := sql.Open("postgres", viper.GetString("kmedia.url"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	boil.SetDB(db)
	//boil.DebugMode = true

	url := viper.GetString("elasticsearch.url")
	es, err := elastic.NewClient(
		elastic.SetURL(viper.GetString("elasticsearch.url")),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10 * time.Second),
		elastic.SetMaxRetries(5),
		elastic.SetErrorLog(log.StandardLogger()),
		//elastic.SetInfoLog(log.StandardLogger()),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Getting the ES version number is quite common, so there's a shortcut
	esversion, err := es.ElasticsearchVersion(url)
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Elasticsearch version %s", esversion)

	langs, reverseLangs, err := getLangs(db)
	if err != nil {
		log.Fatal(err)
	}

	err = recreateIndexes(es, langs)
	if err != nil {
		log.Fatal(err)
	}

	chunkSize := 50

	//log.Info("Loading data from kmedia into elastic search")
	//server_map := make(map[string]string)
	//servers, err := kmedia.Servers(db).All()
	//for _, server := range servers {
	//	server_map[server.Servername] = server.Httpurl.String
	//}

	lcount, err := kmedia.VirtualLessons(db).Count()
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("%d Lessons in Kemdia DB", lcount)
	log.Infof("Expecting %d pages", int(math.Ceil(float64(lcount) / float64(chunkSize))))

	lessons, err := kmedia.VirtualLessons(db).All()
	//lessons, err := kmedia.VirtualLessons(db, OrderBy("created_at desc"), Limit(5)).All()
	if err != nil {
		log.Fatal(err)
	}

	pageCount := 0
	tookSum := 0
	bulkRequest := es.Bulk()
	for lcount, l := range lessons {
		esl := models.Lesson{
			KmediaModel: models.KmediaModel{
				KmediaID: l.ID,
				CreatedAt: l.CreatedAt,
				UpdatedAt: l.UpdatedAt},
			FilmDate: l.FilmDate.Time,
			UserID: l.UserID.Int}

		containers, err := l.ContainersG(Load("FileAssets")).All()
		if err != nil {
			log.Fatal(err)
		}

		//log.Infof("Containers [%d]", len(containers))
		for _, c := range containers {
			description, _ := c.ContainerDescriptions(db, Where("lang_id=?", "ENG")).One()
			esc := models.Container{
				KmediaModel: models.KmediaModel{
					KmediaID: c.ID,
					CreatedAt: c.CreatedAt.Time,
					UpdatedAt: c.UpdatedAt.Time},
				Name: c.Name.String,
				Describable: models.Describable{Description: description.ContainerDesc.String},
				FullDescription: description.Descr.String,
				Filmdate: c.Filmdate.Time,
				LecturerID: c.LecturerID.Int,
				Secure: c.Secure,
				MarkedForMerge: c.MarkedForMerge.Bool,
				SecureChanged: c.SecureChanged.Bool,
				AutoParsed: c.AutoParsed.Bool,
				PlaytimeSecs: c.PlaytimeSecs.Int,
				UserID: c.UserID.Int,
				ForCensorship: c.ForCensorship.Bool,
				OpenedByCensor: c.OpenedByCensor.Bool,
				ClosedByCensor: c.ClosedByCensor.Bool,
				CensorID: c.CensorID.Int,
				Position: c.Position.Int,
			}

			for _, f := range c.R.FileAssets {
				esf := models.FileAsset{
					KmediaModel: models.KmediaModel{
						KmediaID: f.ID,
						CreatedAt: f.CreatedAt.Time,
						UpdatedAt: f.UpdatedAt.Time},
					Name: f.Name.String,
					Lang: reverseLangs[f.LangID.String],
					AssetTypeID: f.AssetTypeID.String,
					Date: f.Date.Time,
					Size: f.Size.Int,
					ServerNameID: f.ServerNameID.String,
					Status: f.Status.String,
					Lastuser: f.Lastuser.String,
					Clicks: f.Clicks.Int,
					Secure: f.Secure.Int,
					PlaytimeSecs: f.PlaytimeSecs.Int,
					UserID: f.UserID.Int,
				}

				description, _ := f.FileFileAssetDescriptions(db, Where("lang_id=?", "ENG")).One()
				if description != nil && description.Filedesc.Valid {
					esf.Describable = models.Describable{Description: description.Filedesc.String}
				}

				esc.FileAssets = append(esc.FileAssets, esf)
			}
			esl.Containers = append(esl.Containers, esc)

			//TODO: Maybe sort containers somehow (position or timestamps) ?
		}

		bulkRequest.Add(elastic.NewBulkIndexRequest().Index(indexName("en")).Type("lesson").Doc(esl))

		// By now we have a full lesson with all details in English.
		// Here we copy everything and replace description languages

		for k, v := range langs {
			if k == "en" {
				continue
			}
			esl2 := models.Lesson{
				KmediaModel: models.KmediaModel{
					KmediaID: esl.KmediaID,
					CreatedAt: esl.CreatedAt,
					UpdatedAt: esl.UpdatedAt},
				FilmDate: esl.FilmDate,
				UserID: esl.UserID, }

			for _, c := range esl.Containers {
				esc := models.Container{
					KmediaModel: models.KmediaModel{
						KmediaID: c.KmediaID,
						CreatedAt: c.CreatedAt,
						UpdatedAt: c.UpdatedAt},
					Name: c.Name,
					Filmdate: c.Filmdate,
					LecturerID: c.LecturerID,
					Secure: c.Secure,
					MarkedForMerge: c.MarkedForMerge,
					SecureChanged: c.SecureChanged,
					AutoParsed: c.AutoParsed,
					PlaytimeSecs: c.PlaytimeSecs,
					UserID: c.UserID,
					ForCensorship: c.ForCensorship,
					OpenedByCensor: c.OpenedByCensor,
					ClosedByCensor: c.ClosedByCensor,
					CensorID: c.CensorID,
					Position: c.Position,
				}
				description, _ := kmedia.ContainerDescriptions(db,
					Where("container_id=? AND lang_id=?", c.KmediaID, v)).One()
				if description != nil && description.ContainerDesc.Valid {
					esc.Description = description.ContainerDesc.String
					esc.FullDescription = description.Descr.String
				} else {
					esc.Description = c.Description
					esc.FullDescription = c.FullDescription
				}

				for _, f := range c.FileAssets {
					esf := models.FileAsset{
						KmediaModel: models.KmediaModel{
							KmediaID: f.KmediaID,
							CreatedAt: f.CreatedAt,
							UpdatedAt: f.UpdatedAt},
						Name: f.Name,
						Lang: f.Lang,
						AssetTypeID: f.AssetTypeID,
						Date: f.Date,
						Size: f.Size,
						ServerNameID: f.ServerNameID,
						Status: f.Status,
						Lastuser: f.Lastuser,
						Clicks: f.Clicks,
						Secure: f.Secure,
						PlaytimeSecs: f.PlaytimeSecs,
						UserID: f.UserID,
					}

					description, _ := kmedia.FileAssetDescriptions(db, Where("file_id=? AND lang_id=?", f.KmediaID, v)).One()
					if description != nil && description.Filedesc.Valid {
						esf.Describable = models.Describable{Description: description.Filedesc.String}
					} else {
						esf.Description = f.Description
					}

					esc.FileAssets = append(esc.FileAssets, esf)
				}

				esl2.Containers = append(esl2.Containers, esc)
			}
			bulkRequest.Add(elastic.NewBulkIndexRequest().Index(indexName(k)).Type("lesson").Doc(esl2))
		}

		if lcount % chunkSize == 0 {
			pageCount++
			log.Infof("Doing bulk request %d", pageCount)
			res, _ := doBulkRequest(bulkRequest)
			if res != nil {
				tookSum += res.Took
			}
		}

		//_, err = es.Index().Index(index).Type("lesson").BodyJson(esl).Do(ctx)
		//_, err := es.Index().Index(index).Type("lesson").BodyJson(l).Do(ctx)
		//j, _ := json.Marshal(l)
		//log.Infof("Lesson: %s", j)
		////_, err := es.Index().Index(index).Type("lesson").BodyString(string(j)).Do(ctx)
		//if err != nil {
		//	log.Fatal(err)
		//}


		//j, _ := json.Marshal(l)
		//log.Infof("Lesson: %s", j)
		//
		//containers, err := l.ContainersG(Load("ContentType"),
		//	Load("ContainerDescriptions"),
		//	Load("FileAssets.FileFileAssetDescriptions")).All()
		//if err != nil {
		//	log.Fatal(err)
		//}
		//
		//log.Infof("Containers [%d]", len(containers))
		//for j := range containers {
		//	c := containers[j]
		//	j, _ := json.Marshal(c)
		//	log.Infof("Container: %s", j)
		//
		//	log.Info("Containers Descriptions")
		//	for k := range c.R.ContainerDescriptions {
		//		d := c.R.ContainerDescriptions[k]
		//		if d.ContainerDesc.Valid && d.ContainerDesc.String != "" {
		//			j, _ := json.Marshal(d)
		//			log.Infof("ContainerDescription: %s", j)
		//		}
		//	}
		//
		//	log.Infof("File Assets [%d]", len(c.R.FileAssets))
		//	for m := range c.R.FileAssets {
		//		f := c.R.FileAssets[m]
		//		j, _ := json.Marshal(f)
		//		log.Infof("File Asset: %s", j)
		//		url := server_map[f.ServerNameID.String] + "/" + f.Name.String
		//		log.Info(url)
		//	}
		//
		//}

	}
	if bulkRequest.NumberOfActions() >= 0 {
		pageCount++
		log.Infof("Doing last bulk request %d with %d actions in it", pageCount, bulkRequest.NumberOfActions())
		res, _ := doBulkRequest(bulkRequest)
		if res != nil {
			tookSum += res.Took
		}
	}

	log.Infof("Avg page took %f", float64(tookSum) / float64(pageCount))

}

func getLangs(db *sql.DB) (map[string]string, map[string]string, error) {
	languages, err := kmedia.Languages(db).All()
	if err != nil {
		return nil, nil, err
	}
	langs := make(map[string]string)
	reverseLangs := make(map[string]string)
	for _, l := range languages {
		langs[l.Locale.String] = l.Code3.String
		reverseLangs[l.Code3.String] = l.Locale.String
	}
	return langs, reverseLangs, nil
}

func indexName(lang string) string {
	return "mdb_" + lang
}

func recreateIndexes(es *elastic.Client, langs map[string]string) error {
	ctx := context.Background()
	for k := range langs {
		name := indexName(k)
		exists, err := es.IndexExists(name).Do(ctx)
		if exists {
			log.Debugf("Index %s already exist, deleting...", name)
			_, err = es.DeleteIndex(name).Do(ctx)
			if err != nil {
				return err
			}
		}

		log.Infof("Creating index: %s", name)
		//mappings, err := ioutil.ReadFile("mappings/mappings_en.json")
		//if err != nil {
		//	log.Fatal(err)
		//}
		//var bodyJson map[string]interface{}
		//if err = json.Unmarshal(mappings, &bodyJson); err != nil {
		//	log.Fatal(err)
		//}
		//_, err = es.CreateIndex(index).BodyJson(bodyJson).Do(ctx)
		_, err = es.CreateIndex(name).Do(ctx)
		if err != nil {
			return err
		}
		_, err = es.IndexPutSettings(name).BodyJson(map[string]interface{}{
			"refresh_interval": "-1",
			"number_of_replicas" : 0,
		}).Do(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func doBulkRequest(bulkRequest *elastic.BulkService) (*elastic.BulkResponse, error) {
	res, err := bulkRequest.Do(context.Background())
	if err != nil {
		log.Fatal("Error doing bulk request", err)
	}
	log.Infof("Done in %d", res.Took)
	if res.Errors {
		failed := res.Failed()
		log.Warnf("%d Documents failed.", len(failed))
		for _, x := range failed {
			j, _ := json.Marshal(x.Error)
			log.Infof("%s", j)
		}
	}
	return res, err
}
