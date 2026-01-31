package indexing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

// LanguageAnalyzerConfig defines how to analyze text for a specific language
type LanguageAnalyzerConfig struct {
	Code           string   // Language code (en, he, ru, etc.)
	Name           string   // Display name
	HunspellLocale string   // Hunspell locale (e.g., "he_IL", empty if not supported)
	NeedsICU       bool     // Whether to create ICU analyzer variant
	NeedsCJK       bool     // Whether to use CJK tokenizer
	StopWords      string   // Stop words list (_english_, _russian_, etc.)
	Stemmer        string   // Stemmer type (english, russian, light_spanish, etc.)
	SynonymFile    string   // Path to synonym file
}

// GetLanguageConfigs returns analyzer configurations for all supported languages
func GetLanguageConfigs() map[string]*LanguageAnalyzerConfig {
	return map[string]*LanguageAnalyzerConfig{
		consts.LANG_ENGLISH: {
			Code:           consts.LANG_ENGLISH,
			Name:           "English",
			HunspellLocale: "", // English uses stemmer, not hunspell
			NeedsICU:       false,
			NeedsCJK:       false,
			StopWords:      "_english_",
			Stemmer:        "english",
			SynonymFile:    "synonyms/en.txt",
		},
		consts.LANG_HEBREW: {
			Code:           consts.LANG_HEBREW,
			Name:           "Hebrew",
			HunspellLocale: "he_IL",
			NeedsICU:       true, // Hebrew benefits from ICU
			NeedsCJK:       false,
			StopWords:      "",
			Stemmer:        "",
			SynonymFile:    "synonyms/he.txt",
		},
		consts.LANG_RUSSIAN: {
			Code:           consts.LANG_RUSSIAN,
			Name:           "Russian",
			HunspellLocale: "",
			NeedsICU:       false,
			NeedsCJK:       false,
			StopWords:      "_russian_",
			Stemmer:        "russian",
			SynonymFile:    "synonyms/ru.txt",
		},
		consts.LANG_SPANISH: {
			Code:           consts.LANG_SPANISH,
			Name:           "Spanish",
			HunspellLocale: "",
			NeedsICU:       false,
			NeedsCJK:       false,
			StopWords:      "_spanish_",
			Stemmer:        "light_spanish",
			SynonymFile:    "synonyms/es.txt",
		},
		// TODO: Add remaining 34 languages from consts.ALL_KNOWN_LANGS
		// For now, languages without specific config use "standard" analyzer
	}
}

// GenerateMapping creates a complete ES9 mapping for a specific language
func GenerateMapping(langCode string) (map[string]interface{}, error) {
	langConfig := GetLanguageConfigs()[langCode]
	if langConfig == nil {
		// Fallback for languages without specific config
		langConfig = &LanguageAnalyzerConfig{
			Code:     langCode,
			Name:     langCode,
			NeedsICU: false,
		}
	}

	mapping := map[string]interface{}{
		"settings": generateSettings(langConfig),
		"mappings": generateMappings(langConfig),
	}

	return mapping, nil
}

// generateSettings creates the analysis configuration for a language
func generateSettings(lang *LanguageAnalyzerConfig) map[string]interface{} {
	settings := map[string]interface{}{
		"index": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
			"analysis": map[string]interface{}{
				"char_filter": generateCharFilters(lang),
				"filter":      generateFilters(lang),
				"analyzer":    generateAnalyzers(lang),
			},
		},
	}

	return settings
}

// generateCharFilters creates character filters (same for all languages)
func generateCharFilters(lang *LanguageAnalyzerConfig) map[string]interface{} {
	return map[string]interface{}{
		"quotes": map[string]interface{}{
			"type": "mapping",
			"mappings": []string{
				// Hebrew quotes
				"\\u05F3\\u05F3=>\\u0029", // Geresh doubled
				"\\u059C\\u059C=>\\u0029",
				"\\u059D\\u059D=>\\u0029",
				"\\u05F3=>\\u0027", // Single geresh
				"\\u059C=>\\u0027",
				"\\u059D=>\\u0027",
				"\\u05F4=>", // Gershayim - remove

				// English quotes
				"\\u0027\\u0027=>\\u0029",
				"\\u0091\\u0091=>\\u0029",
				"\\u0092\\u0092=>\\u0029",
				"\\u2018\\u2018=>\\u0029",
				"\\u2019\\u2019=>\\u0029",
				"\\u201B\\u201B=>\\u0029",
				"\\u0091=>\\u0027",
				"\\u0092=>\\u0027",
				"\\u2018=>\\u0027",
				"\\u2019=>\\u0027",
				"\\u201B=>\\u0027",
				"\\u0022=>",
				"\\u201C=>",
				"\\u201D=>",
			},
		},
	}
}

// generateFilters creates token filters based on language config
func generateFilters(lang *LanguageAnalyzerConfig) map[string]interface{} {
	filters := make(map[string]interface{})

	// Synonym graph (all languages)
	filters["synonym_graph"] = map[string]interface{}{
		"type":      "synonym_graph",
		"tokenizer": "keyword",
		"synonyms":  []string{}, // Load from file in production
	}

	// Hunspell (if supported)
	if lang.HunspellLocale != "" {
		filters[lang.HunspellLocale] = map[string]interface{}{
			"type":   "hunspell",
			"locale": lang.HunspellLocale,
			"dedup":  true,
		}
	}

	// ICU (if needed)
	if lang.NeedsICU {
		filters["icu_normalizer"] = map[string]interface{}{
			"type": "icu_normalizer",
			"name": "nfkc_cf",
		}
		filters["icu_folding"] = map[string]interface{}{
			"type": "icu_folding",
		}
	}

	// Stop words (if specified)
	if lang.StopWords != "" {
		filters[lang.Code+"_stop"] = map[string]interface{}{
			"type":      "stop",
			"stopwords": lang.StopWords,
		}
	}

	// Stemmer (if specified)
	if lang.Stemmer != "" {
		filters[lang.Code+"_stemmer"] = map[string]interface{}{
			"type":     "stemmer",
			"language": lang.Stemmer,
		}

		// Possessive stemmer for English
		if lang.Code == consts.LANG_ENGLISH {
			filters["english_possessive_stemmer"] = map[string]interface{}{
				"type":     "stemmer",
				"language": "possessive_english",
			}
		}
	}

	return filters
}

// generateAnalyzers creates analyzer configurations based on language
func generateAnalyzers(lang *LanguageAnalyzerConfig) map[string]interface{} {
	analyzers := make(map[string]interface{})

	// Choose tokenizer
	tokenizer := "standard"
	if lang.NeedsCJK {
		tokenizer = "cjk"
	}

	icuTokenizer := "icu_tokenizer"

	// Build filter chain based on language capabilities
	var filters []string

	// PRIMARY ANALYZER: Language-specific with synonyms
	if lang.HunspellLocale != "" {
		// Hebrew-like: Hunspell-based
		filters = []string{"synonym_graph", lang.HunspellLocale}
		analyzers[lang.Code+"_hunspell"] = map[string]interface{}{
			"tokenizer":   tokenizer,
			"char_filter": []string{"quotes"},
			"filter":      filters,
		}
	} else if lang.Stemmer != "" {
		// English-like: Stemmer-based
		filters = []string{}
		if lang.Code == consts.LANG_ENGLISH {
			filters = append(filters, "english_possessive_stemmer")
		}
		filters = append(filters, lang.Code+"_stop", lang.Code+"_stemmer", "synonym_graph")

		analyzers[lang.Code+"_stemmer"] = map[string]interface{}{
			"tokenizer":   tokenizer,
			"char_filter": []string{"quotes"},
			"filter":      filters,
		}
	} else {
		// Fallback: Standard analyzer with synonyms
		analyzers[lang.Code+"_standard"] = map[string]interface{}{
			"tokenizer":   tokenizer,
			"char_filter": []string{"quotes"},
			"filter":      []string{"synonym_graph"},
		}
	}

	// ICU ANALYZER (if needed)
	if lang.NeedsICU {
		icuFilters := []string{"icu_normalizer", "icu_folding", "synonym_graph"}
		analyzers[lang.Code+"_icu"] = map[string]interface{}{
			"tokenizer":   icuTokenizer,
			"char_filter": []string{"quotes"},
			"filter":      icuFilters,
		}

		// ICU + Hunspell combo (for Hebrew)
		if lang.HunspellLocale != "" {
			icuHunspellFilters := []string{"icu_normalizer", "synonym_graph", lang.HunspellLocale}
			analyzers[lang.Code+"_icu_hunspell"] = map[string]interface{}{
				"tokenizer":   icuTokenizer,
				"char_filter": []string{"quotes"},
				"filter":      icuHunspellFilters,
			}
		}
	}

	return analyzers
}

// generateMappings creates the properties section (same structure, different analyzer names)
func generateMappings(lang *LanguageAnalyzerConfig) map[string]interface{} {
	// Determine primary analyzer name
	primaryAnalyzer := lang.Code + "_standard"
	if lang.HunspellLocale != "" {
		primaryAnalyzer = lang.Code + "_hunspell"
	} else if lang.Stemmer != "" {
		primaryAnalyzer = lang.Code + "_stemmer"
	}

	// Determine secondary analyzer (ICU variant)
	icuAnalyzer := ""
	if lang.NeedsICU {
		icuAnalyzer = lang.Code + "_icu"
	}

	// Determine combo analyzer (ICU + morphology)
	comboAnalyzer := ""
	if lang.NeedsICU && lang.HunspellLocale != "" {
		comboAnalyzer = lang.Code + "_icu_hunspell"
	}

	// Build multi-field text field
	textField := func(fieldName string) map[string]interface{} {
		field := map[string]interface{}{
			"type":     "text",
			"analyzer": "standard",
			"fields": map[string]interface{}{
				"language": map[string]interface{}{
					"type":     "text",
					"analyzer": primaryAnalyzer,
				},
			},
		}

		// Add ICU variant if supported
		if icuAnalyzer != "" {
			field["fields"].(map[string]interface{})["icu"] = map[string]interface{}{
				"type":     "text",
				"analyzer": icuAnalyzer,
			}
		}

		// Add combo variant if supported
		if comboAnalyzer != "" {
			field["fields"].(map[string]interface{})["icu_hunspell"] = map[string]interface{}{
				"type":     "text",
				"analyzer": comboAnalyzer,
			}
		}

		return field
	}

	properties := map[string]interface{}{
		// Metadata fields
		"result_type": map[string]interface{}{"type": "keyword"},
		"index_date":  map[string]interface{}{"type": "date", "format": "strict_date"},
		"mdb_uid":     map[string]interface{}{"type": "keyword"},
		"typed_uids":  map[string]interface{}{"type": "keyword"},
		"filter_values": map[string]interface{}{"type": "keyword"},

		// Searchable text fields
		"title":      textField("title"),
		"full_title": textField("full_title"),
		"description": textField("description"),
		"content":    textField("content"),
		"full_content": textField("full_content"),

		// Completion suggester
		"title_suggest": map[string]interface{}{
			"type":     "completion",
			"analyzer": "standard",
			"contexts": []map[string]interface{}{
				{
					"name": "result_type",
					"type": "category",
					"path": "result_type",
				},
			},
			"fields": map[string]interface{}{
				"language": map[string]interface{}{
					"type":     "completion",
					"analyzer": primaryAnalyzer,
					"contexts": []map[string]interface{}{
						{
							"name": "result_type",
							"type": "category",
							"path": "result_type",
						},
					},
				},
			},
		},

		// Date field
		"effective_date": map[string]interface{}{
			"type":   "date",
			"format": "strict_date",
		},
	}

	return map[string]interface{}{
		"properties": properties,
	}
}

// GenerateAllMappings generates mapping files for all languages
func GenerateAllMappings(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	for _, langCode := range consts.ALL_KNOWN_LANGS {
		mapping, err := GenerateMapping(langCode)
		if err != nil {
			return fmt.Errorf("generate mapping for %s: %w", langCode, err)
		}

		filename := filepath.Join(outputDir, fmt.Sprintf("results-%s.json", langCode))
		data, err := json.MarshalIndent(mapping, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal mapping for %s: %w", langCode, err)
		}

		if err := os.WriteFile(filename, data, 0644); err != nil {
			return fmt.Errorf("write mapping file for %s: %w", langCode, err)
		}

		fmt.Printf("Generated mapping: %s\n", filename)
	}

	return nil
}
