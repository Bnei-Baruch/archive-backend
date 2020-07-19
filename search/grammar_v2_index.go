package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/bindata"
	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type GrammarRule struct {
	HitType      string          `json:"hit_type"`
	Intent       string          `json:"intent"`
	Variables    []string        `json:"variables,omitempty"`
	Values       []string        `json:"values,omitempty"`
	Rules        []string        `json:"rules"`
	RulesSuggest es.SuggestField `json:"rules_suggest"`
}

const (
	GRAMMARS_INDEX_BASE_NAME = "grammars"
)

func GrammarIndexNameFunc(indexDate string) es.IndexNameByLang {
	return func(lang string) string {
		return GrammarIndexName(lang, indexDate)
	}
}

func GrammarIndexName(lang string, indexDate string) string {
	if indexDate == "" {
		return fmt.Sprintf("prod_%s_%s", GRAMMARS_INDEX_BASE_NAME, lang)
	} else {
		return fmt.Sprintf("prod_%s_%s_%s", GRAMMARS_INDEX_BASE_NAME, lang, indexDate)
	}
}

func GrammarIndexNameForServing(lang string) string {
	grammarIndexDate := viper.GetString("elasticsearch.grammar-index-date")
	// When grammarIndexDate empty will use alias, otherwise config flag.
	return GrammarIndexName(lang, grammarIndexDate)
}

func DeleteGrammarIndex(esc *elastic.Client, indexDate string) error {
	for _, lang := range consts.ALL_KNOWN_LANGS {
		name := GrammarIndexName(lang, indexDate)
		exists, err := esc.IndexExists(name).Do(context.TODO())
		if err != nil {
			return err
		}
		if exists {
			res, err := esc.DeleteIndex(name).Do(context.TODO())
			if err != nil {
				return errors.Wrap(err, "Delete index")
			}
			if !res.Acknowledged {
				return errors.Errorf("Index deletion wasn't acknowledged: %s", name)
			}
		}
	}
	return nil
}

func CreateGrammarIndex(esc *elastic.Client, indexDate string) error {
	for _, lang := range consts.ALL_KNOWN_LANGS {
		name := GrammarIndexName(lang, indexDate)
		// Do nothing if index already exists.
		exists, err := esc.IndexExists(name).Do(context.TODO())
		log.Debugf("Create index, exists: %t.", exists)
		if err != nil {
			return errors.Wrapf(err, "Create index, lang: %s, name: %s.", lang, name)
		}
		if exists {
			log.Debugf("Index already exists (%+v), skipping.", name)
			continue
		}

		definition := fmt.Sprintf("data/es/mappings/%s/%s-%s.json", GRAMMARS_INDEX_BASE_NAME, GRAMMARS_INDEX_BASE_NAME, lang)
		// Read mappings and create index
		mappings, err := bindata.Asset(definition)
		if err != nil {
			return errors.Wrapf(err, "Failed loading mapping %s", definition)
		}
		var bodyJson map[string]interface{}
		if err = json.Unmarshal(mappings, &bodyJson); err != nil {
			return errors.Wrap(err, "json.Unmarshal")
		}

		// Create index.
		res, err := esc.CreateIndex(name).BodyJson(bodyJson).Do(context.TODO())
		if err != nil {
			return errors.Wrap(err, "Create index")
		}
		if !res.Acknowledged {
			return errors.Errorf("Index creation wasn't acknowledged: %s", name)
		}
		log.Debugf("Created index: %+v", name)
	}
	return nil
}

type CrossIter struct {
	index  int
	size   int
	values [][]string
}

func CreateCrossIter(values [][]string) *CrossIter {
	size := 1
	for i := range values {
		size *= len(values[i])
	}
	return &CrossIter{index: 0, size: size, values: values}
}

func (ci *CrossIter) Next() bool {
	if ci.index >= ci.size {
		return false
	}
	ci.index++
	return true
}

func (ci *CrossIter) Values() []string {
	ret := []string(nil)
	index := ci.index
	for i := range ci.values {
		offset := index % len(ci.values[i])
		ret = append(ret, ci.values[i][offset])
		index /= len(ci.values[i])
	}
	return ret
}

func IndexGrammars(esc *elastic.Client, indexDate string, grammars GrammarsV2, variables VariablesV2, cm cache.CacheManager) error {
	if err := DeleteGrammarIndex(esc, indexDate); err != nil {
		return err
	}
	if err := CreateGrammarIndex(esc, indexDate); err != nil {
		return err
	}

	log.Infof("Indexing %d grammars.", len(grammars))
	for lang, grammarsByIntent := range grammars {
		name := GrammarIndexName(lang, indexDate)
		bulkService := elastic.NewBulkService(esc).Index(name)
		log.Infof("Indexing %d intents for %s.", len(grammarsByIntent), lang)
		for intent, grammar := range grammarsByIntent {
			log.Infof("Indexing %d variable sets for intet \"%s\".", len(grammar.Patterns), intent)
			for variablesSetAsString, rules := range grammar.Patterns {
				if variablesSetAsString == "" {
					assignedRulesSuggest := []string{}
					for i := range rules {
						assignedRulesSuggest = append(assignedRulesSuggest, es.Suffixes(rules[i])...)
					}
					rule := GrammarRule{
						HitType:      grammar.HitType,
						Intent:       intent,
						Rules:        rules,
						RulesSuggest: es.SuggestField{es.Unique(assignedRulesSuggest), float64(consts.ES_GRAMMAR_SUGGEST_DEFAULT_WEIGHT)},
						Variables:    []string{},
						Values:       []string{},
					}
					bulkService.Add(elastic.NewBulkIndexRequest().Index(name).Type("grammars").Doc(rule))
				} else {

					// List of variables: ["$Year", "$ConventionLocation"]
					variablesSet := VariablesFromString(variablesSetAsString)

					// For better results, we don't add suggestions for combinations of 'holiday' term with the holiday name (we do add suggest for 'holiday' term with year).
					// 	e. g. searchig 'Hanukkah' brings more and better results than 'holiday Hanukkah'.
					addSuggest := intent != consts.GRAMMAR_INTENT_LANDING_PAGE_HOLIDAYS || (len(variablesSet) <= 2 && (variablesSet[0] == consts.VAR_YEAR || variablesSet[1] == consts.VAR_YEAR))

					// Set of possible variable values: [["2000", "2001", ...], ["Moscow", "Tel Aviv", "New York", ...]]
					variablesValues := [][]string(nil)
					for i := range variablesSet {
						variablesValues = append(variablesValues, utils.StringMapOrderedKeys(variables[variablesSet[i]][lang]))
					}
					log.Infof("Cross iterating over %+v", variablesValues)
					// Iterate over each pair of values, e.g., ["2018", "Moscow"], ["2019", "Moscow"], ..., ["2018", "Tel Aviv"], ...
					for valueIter := CreateCrossIter(variablesValues); valueIter.Next(); {
						variableValues := valueIter.Values()
						log.Infof("values set: %+v", variableValues)
						assignedRules := []string(nil)
						for i := range rules {
							assignedRule := rules[i]
							// For set of values: ["2018", "Moscow"] provide list of phrases:
							// [["2018", "Two thousand and eigheen"], ["Moscow", "Russian, Moscow"]]
							variableValuesPhrases := [][]string(nil)
							for j := range variableValues {
								variableValuesPhrases = append(variableValuesPhrases, variables[variablesSet[j]][lang][variableValues[j]])
							}
							// Iterate over different pheases for each value, see |variableValuesPhrases| variable.
							for phrasesIter := CreateCrossIter(variableValuesPhrases); phrasesIter.Next(); {
								assignValues := phrasesIter.Values()
								for j := range assignValues {
									assignedRule = strings.Replace(assignedRule, variablesSet[j], assignValues[j], -1)
								}
								assignedRules = append(assignedRules, assignedRule)
							}
						}

						assignedRulesSuggest := []string{}
						if addSuggest {
							for i := range assignedRules {
								assignedRulesSuggest = append(assignedRulesSuggest, es.Suffixes(assignedRules[i])...)
							}
							for i := range assignedRulesSuggest {
								if assignedRulesSuggest[i] == "" {
									log.Infof("NNN: %+v", assignedRulesSuggest[i])
								}
							}
							log.Infof("Rules suggest: [%s]", strings.Join(assignedRulesSuggest, "|"))
						}

						vMap := make(map[string][]string)
						for i := range variablesSet {
							vMap[variablesSet[i]] = []string{variableValues[i]}
						}
						if GrammarVariablesMatch(intent, vMap, cm) {
							rule := GrammarRule{
								HitType:      grammar.HitType,
								Intent:       intent,
								Rules:        assignedRules,
								RulesSuggest: es.SuggestField{es.Unique(assignedRulesSuggest), float64(100)},
								Variables:    variablesSet,
								Values:       variableValues,
							}
							bulkService.Add(elastic.NewBulkIndexRequest().Index(name).Type("grammars").Doc(rule))
						}
					}
				}
			}
		}
		if bulkRes, err := bulkService.Do(context.TODO()); err != nil {
			return err
		} else {
			for _, itemMap := range bulkRes.Items {
				for _, res := range itemMap {
					if res.Error != nil {
						log.Infof("Error: %+v", res.Error)
						log.Infof("Res: %+v", res)
					}
				}
			}
		}
	}

	return nil
}
