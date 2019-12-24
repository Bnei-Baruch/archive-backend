package search

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v6"

	"github.com/Bnei-Baruch/archive-backend/bindata"
	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type SuggestField struct {
	Input  []string `json:"input"`
	Weight float64  `json:"weight"`
}

type GrammarRule struct {
	HitType      string       `json:"hit_type"`
	Intent       string       `json:"intent"`
	Variables    []string     `json:"variables,omitempty"`
	Values       []string     `json:"values,omitempty"`
	Rules        []string     `json:"rules"`
	RulesSuggest SuggestField `json:"rules_suggest"`
	FilterValues []string     `json:"filter_values"`
}

func GrammarIndexName(lang string) string {
	return fmt.Sprintf("prod_%s_%s", consts.GRAMMARS_INDEX_BASE_NAME, lang)
}

func DeleteGrammarIndex(esc *elastic.Client) error {
	for _, lang := range consts.ALL_KNOWN_LANGS {
		name := GrammarIndexName(lang)
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

func CreateGrammarIndex(esc *elastic.Client) error {
	for _, lang := range consts.ALL_KNOWN_LANGS {
		name := GrammarIndexName(lang)
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

		definition := fmt.Sprintf("data/es/mappings/%s/%s-%s.json", consts.GRAMMARS_INDEX_BASE_NAME, consts.GRAMMARS_INDEX_BASE_NAME, lang)
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

func IndexGrammars(esc *elastic.Client, grammars GrammarsV2, variables VariablesV2, cm cache.CacheManager) error {
	if err := DeleteGrammarIndex(esc); err != nil {
		return err
	}
	if err := CreateGrammarIndex(esc); err != nil {
		return err
	}

	log.Infof("Indexing %d grammars.", len(grammars))
	for lang, grammarsByIntent := range grammars {
		name := GrammarIndexName(lang)
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
					filterValues := []string{consts.GRAMMAR_TYPE_FULL, consts.HIT_TYPE_TO_GRAMMAR_TYPE[grammar.HitType]}
					rule := GrammarRule{
						HitType:      grammar.HitType,
						Intent:       intent,
						Rules:        rules,
						RulesSuggest: SuggestField{es.Unique(assignedRulesSuggest), float64(100)},
						Variables:    []string{},
						Values:       []string{},
						FilterValues: filterValues,
					}
					bulkService.Add(elastic.NewBulkIndexRequest().Index(name).Type("grammars").Doc(rule))
				} else {
					// List of variables: ["$Year", "$ConventionLocation"]
					variablesSet := VariablesFromString(variablesSetAsString)
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
						for i := range assignedRules {
							assignedRulesSuggest = append(assignedRulesSuggest, es.Suffixes(assignedRules[i])...)
						}
						for i := range assignedRulesSuggest {
							if assignedRulesSuggest[i] == "" {
								log.Infof("NNN: %+v", assignedRulesSuggest[i])
							}
						}
						log.Infof("Rules suggest: [%s]", strings.Join(assignedRulesSuggest, "|"))

						vMap := make(map[string][]string)
						for i := range variablesSet {
							vMap[variablesSet[i]] = []string{variableValues[i]}
						}
						filterValues := []string{consts.HIT_TYPE_TO_GRAMMAR_TYPE[grammar.HitType]}
						if _, ok := vMap["$Text"]; ok {
							filterValues = append(filterValues, consts.GRAMMAR_TYPE_PARTIAL)
						} else {
							filterValues = append(filterValues, consts.GRAMMAR_TYPE_FULL)
						}
						if GrammarVariablesMatch(intent, vMap, cm) {
							rule := GrammarRule{
								HitType:      grammar.HitType,
								Intent:       intent,
								Rules:        assignedRules,
								RulesSuggest: SuggestField{es.Unique(assignedRulesSuggest), float64(100)},
								Variables:    variablesSet,
								Values:       variableValues,
								FilterValues: filterValues,
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
