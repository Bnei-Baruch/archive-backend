package es

import (
	"context"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/golang/sync/errgroup"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type TagIndexUOW struct {
	Tag *mdbmodels.Tag
}

func IndexClassifications() {
	clock := Init()

	for i := range consts.ALL_KNOWN_LANGS {
		lang := consts.ALL_KNOWN_LANGS[i]
		name := IndexName(consts.ES_CLASSIFICATIONS_INDEX, lang)
		mappings := fmt.Sprintf("data/es/mappings/classification/classification-%s.json", lang)
		utils.Must(recreateIndex(name, mappings))
	}

	ctx := context.Background()
	utils.Must(indexTags(ctx))
	utils.Must(indexSources(ctx))

	for i := range consts.ALL_KNOWN_LANGS {
		lang := consts.ALL_KNOWN_LANGS[i]
		name := IndexName(consts.ES_CLASSIFICATIONS_INDEX, lang)
		utils.Must(finishIndexing(name))
	}

	Shutdown()
	log.Info("Success")
	log.Infof("Total run time: %s", time.Now().Sub(clock).String())
}

func indexTags(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	tagsCH := make(chan *mdbmodels.Tag)
	g.Go(func() error {
		defer close(tagsCH)

		count, err := mdbmodels.Tags(db).Count()
		if err != nil {
			return errors.Wrap(err, "Count tags in mdb")
		}
		log.Infof("%d tags in MDB", count)

		tags, err := mdbmodels.Tags(db, qm.Load("TagI18ns")).All()
		if err != nil {
			return errors.Wrap(err, "Fetch tags from mdb")
		}

		for i := range tags {
			tag := tags[i]

			// we don't index root nodes
			if !tag.ParentID.Valid {
				log.Infof("skipping root tag %s", tag.UID)
				continue
			}

			select {
			case tagsCH <- tag:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	})

	for i := 1; i <= 5; i++ {
		g.Go(func() error {
			for tag := range tagsCH {
				if err := indexTag(tag); err != nil {
					log.Errorf("Index tag error: %s", err.Error())
					return err
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

func indexTag(t *mdbmodels.Tag) error {
	//log.Infof("Indexing tag %s", t.UID)

	// create documents in each language with available translation
	i18nMap := make(map[string]Classification)
	for i := range t.R.TagI18ns {
		i18n := t.R.TagI18ns[i]
		if i18n.Label.Valid && i18n.Label.String != "" {
			i18nMap[i18n.Language] = Classification{
				MDB_UID:     t.UID,
				Type:        "tag",
				Name:        i18n.Label.String,
				NameSuggest: i18n.Label.String,
			}
		}
	}

	// index each document in its language index
	for k, v := range i18nMap {
		name := IndexName(consts.ES_CLASSIFICATIONS_INDEX, k)
		resp, err := esc.Index().
			Index(name).
			Type("tags").
			BodyJson(v).
			Do(context.TODO())
		if err != nil {
			return errors.Wrapf(err, "Index tag %s %s", name, t.UID)
		}
		if !resp.Created {
			return errors.Errorf("Not created: tag %s %s", name, t.UID)
		}
	}

	return nil
}

func indexSources(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	sourcesCH := make(chan *mdbmodels.Source)
	g.Go(func() error {
		defer close(sourcesCH)

		count, err := mdbmodels.Sources(db).Count()
		if err != nil {
			return errors.Wrap(err, "Count sources in mdb")
		}
		log.Infof("%d sources in MDB", count)

		sources, err := mdbmodels.Sources(db, qm.Load("SourceI18ns")).All()
		if err != nil {
			return errors.Wrap(err, "Fetch sources from mdb")
		}

		for i := range sources {
			select {
			case sourcesCH <- sources[i]:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	})

	for i := 1; i <= 5; i++ {
		g.Go(func() error {
			for source := range sourcesCH {
				if err := indexSource(source); err != nil {
					log.Errorf("Index source error: %s", err.Error())
					return err
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

func indexSource(s *mdbmodels.Source) error {
	//log.Infof("Indexing source %s", s.UID)

	// create documents in each language with available translation
	i18nMap := make(map[string]Classification)
	for i := range s.R.SourceI18ns {
		i18n := s.R.SourceI18ns[i]
		if i18n.Name.Valid && i18n.Name.String != "" {
			x := Classification{
				MDB_UID:     s.UID,
				Type:        "source",
				Name:        i18n.Name.String,
				NameSuggest: i18n.Name.String,
			}
			if i18n.Description.Valid && i18n.Description.String != "" {
				x.Description = i18n.Description.String
				x.DescriptionSuggest = i18n.Description.String
			}
			i18nMap[i18n.Language] = x
		}
	}

	// index each document in its language index
	for k, v := range i18nMap {
		name := IndexName(consts.ES_CLASSIFICATIONS_INDEX, k)
		resp, err := esc.Index().
			Index(name).
			Type("sources").
			BodyJson(v).
			Do(context.TODO())
		if err != nil {
			return errors.Wrapf(err, "Index source %s %s", name, s.UID)
		}
		if !resp.Created {
			return errors.Errorf("Not created: source %s %s", name, s.UID)
		}
	}

	return nil
}
