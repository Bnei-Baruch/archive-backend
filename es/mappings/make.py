#!/usr/bin/python2

# TODO: Rewrite this into go.
# This file creates all mapping for Elastic. It is required because some
# features are supported for some languages but not others. Also specific
# languages need specific treatment such as transliteration for cyrillic other
# specific tokenization for CJK.

# Mapping generated here requires the following Elasticsearch plugins:
#   https://www.elastic.co/guide/en/elasticsearch/guide/current/hunspell.html
#   To install download: he_IL.aff, he_IL.dic, settings.yml files from
#   https://github.com/elastic/hunspell/tree/master/dicts/he_IL
#   and put under: elasticsearch-6.3.0/config/hunspell/he_IL
#
#
# Deprecated plugins (already not in use):
# Hebrew analyzer plugin:
#   https://github.com/synhershko/elasticsearch-analysis-hebrew
# Phonetic plugin:
#   https://www.elastic.co/guide/en/elasticsearch/plugins/current/analysis-phonetic.html
#   sudo bin/elasticsearch-plugin install analysis-phonetic
# ICU plugin (for transliteration):
#   sudo bin/elasticsearch-plugin install analysis-icu

import json
import os

# List of all languages
ENGLISH = "en"
HEBREW = "he"
RUSSIAN = "ru"
SPANISH = "es"
ITALIAN = "it"
GERMAN = "de"
DUTCH = "nl"
FRENCH = "fr"
PORTUGUESE = "pt"
TURKISH = "tr"
POLISH = "pl"
ARABIC = "ar"
HUNGARIAN = "hu"
FINNISH = "fi"
LITHUANIAN = "lt"
JAPANESE = "ja"
BULGARIAN = "bg"
GEORGIAN = "ka"
NORWEGIAN = "no"
SWEDISH = "sv"
CROATIAN = "hr"
CHINESE = "zh"
PERSIAN = "fa"
ROMANIAN = "ro"
HINDI = "hi"
UKRAINIAN = "ua"
MACEDONIAN = "mk"
SLOVENIAN = "sl"
LATVIAN = "lv"
SLOVAK = "sk"
CZECH = "cs"

# Lang groups
ALL = 'ALL_LANGS'
CYRILLIC = 'CYRILLIC'
CJK = 'CJK'
LANG_GROUPS = {
  ALL: [
    ENGLISH, HEBREW, RUSSIAN, SPANISH, ITALIAN, GERMAN, DUTCH, FRENCH,
    PORTUGUESE, TURKISH, POLISH, ARABIC, HUNGARIAN, FINNISH, LITHUANIAN,
    JAPANESE, BULGARIAN, GEORGIAN, NORWEGIAN, SWEDISH, CROATIAN, CHINESE,
    PERSIAN, ROMANIAN, HINDI, MACEDONIAN, SLOVENIAN, LATVIAN, SLOVAK,
    CZECH, UKRAINIAN,
  ],
  CYRILLIC: [RUSSIAN, BULGARIAN, MACEDONIAN, UKRAINIAN],
  CJK: [CHINESE, JAPANESE],
}

# Units indexing
LanguageAnalyzer = {
  ENGLISH: "english",
  HEBREW: "he",
  RUSSIAN: "russian",
  SPANISH: "spanish",
  ITALIAN: "italian",
  GERMAN: "german",
  DUTCH: "dutch",
  FRENCH: "french",
  PORTUGUESE: "portuguese",
  TURKISH: "turkish",
  POLISH: "standard",
  ARABIC: "arabic",
  HUNGARIAN: "hungarian",
  FINNISH: "finnish",
  LITHUANIAN: "lithuanian",
  JAPANESE: "cjk",
  BULGARIAN: "bulgarian",
  GEORGIAN: "standard",
  NORWEGIAN: "norwegian",
  SWEDISH: "swedish",
  CROATIAN: "standard",
  CHINESE: "cjk",
  PERSIAN: "persian",
  ROMANIAN: "romanian",
  HINDI: "hindi",
  UKRAINIAN: "standard",
  MACEDONIAN: "standard",
  SLOVENIAN: "standard",
  LATVIAN: "latvian",
  SLOVAK: "standard",
  CZECH: "czech",
}

# Phonetic analyzer
BEIDER_MORSE_LANGUAGESET = {
  CYRILLIC: 'cyrillic',
  ENGLISH: 'english',
  FRENCH: 'french',
  GERMAN: 'german',
  HEBREW: 'hebrew',
  HUNGARIAN: 'hungarian',
  POLISH: 'polish',
  ROMANIAN: 'romanian',
  RUSSIAN: 'russian',
  SPANISH: 'spanish',
}


def BeiderMorseLanguageset(lang):
  if lang in BEIDER_MORSE_LANGUAGESET:
    return BEIDER_MORSE_LANGUAGESET[lang]
  elif lang in LANG_GROUPS[CYRILLIC]:
    return BEIDER_MORSE_LANGUAGESET[CYRILLIC]
  else:
    return None


def IsCyrillic(lang, something):
  return something if lang in LANG_GROUPS[CYRILLIC] else None


SETTINGS = {
  "index": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "analyzer": {
        "he": {
          "tokenizer": "standard",
          "filter": [
            "he_IL"
          ],
          "char_filter": [
            "quotes"
          ]
        },
        # "phonetic_analyzer": {
        #   "tokenizer": "standard",
        #   "char_filter": ["quotes"],
        #   "filter": [
        #     "standard",
        #     "lowercase",
        #     lambda lang: IsCyrillic(lang, 'icu_transliterate'),
        #     "custom_phonetic",
        #   ],
        # },
      },
      "char_filter": {
        "quotes": {
          "type": "mapping",
          "mappings": [
            "\\u0091=>\\u0027",
            "\\u0092=>\\u0027",
            "\\u2018=>\\u0027",
            "\\u2019=>\\u0027",
            "\\u201B=>\\u0027",
            "\\u0022=>",
            "\\u201C=>",
            "\\u201D=>",
          ],
        },
      },
      "filter": {
        "he_IL": {
          "type": "hunspell",
          "locale": "he_IL",
          "dedup": True,
        },
        # "icu_transliterate": lambda lang: IsCyrillic(lang, {
        #   "type": "icu_transform",
        #   "id": "Any-Latin; NFD; [:Nonspacing Mark:] Remove; NFC",
        # }),
        # "custom_phonetic": {
        #   "type": "phonetic",
        #   "encoder": "beider_morse",
        #   "replace": True,
        #   "languageset": BeiderMorseLanguageset,
        # },
      },
    },
  },
}


RESULTS_TEMPLATE = {
  # "settings": {
  #   "index": {
  #     "number_of_shards": 1,
  #     "number_of_replicas": 0,
  #   },
  # },
  "settings": SETTINGS,
  "mappings": {
    "result": {
      "dynamic": "strict",
      "properties": {
        # Document type, unit, collection, source, tag.
        "result_type": {
          "type": "keyword",
        },
        "mdb_uid": {
          "type": "keyword",
        },
        # Typed uids, are list of entities (uid and entity type) in MDB that
        # this document depends on. For example: "content_unit:lHDLZWxq",
        # "file:0uzDZVqV". We use this list to reindex the document if one
        # the items in this list changes.
        "typed_uids": {
          "type": "keyword",
        },
        # List of keywords in format filter:value are required for correct
        # filtering of this document by all different filters. Time is handled
        # by effective_date.
        # For example: content_type:DAILY_LESSON or tag:0db5BBS3
        "filter_values": {
          "type": "keyword",
        },
        # Title, Description and Content are the typical result fields which
        # should have the same tf/idf across all different retult types such
        # as units, collections, sources, topics and others to follow.
        "title": {
          "type": "text",
          "analyzer": "standard",
          "fields": {
            "language": {
              "type": "text",
              "analyzer": lambda lang: LanguageAnalyzer[lang],
            }
          }
        },
        "description": {
          "type": "text",
          "analyzer": "standard",
          "fields": {
            "language": {
              "type": "text",
              "analyzer": lambda lang: LanguageAnalyzer[lang],
            },
          },
        },
        "content": {
          "type": "text",
          "analyzer": "standard",
          "fields": {
            "language": {
              "type": "text",
              "analyzer": lambda lang: LanguageAnalyzer[lang],
            },
          },
        },

        # Suggest field for autocomplete.
        "title_suggest": {
          "type": "completion",
          "analyzer": lambda lang: LanguageAnalyzer[lang],
          "contexts": [
            {
              "name": "result_type",
              "type": "category",
              "path": "result_type",
            },
          ],
        },

        # Content unit specific fields.
        "effective_date": {
          "type": "date",
          "format": "strict_date",
        },
      }
    }
  }
}


SEARCH_LOGS_TEMPLATE = {
    "mappings": {
        "search_logs": {
            # We use dynamic mappinng only for search logs.
            "dynamic_templates": [
                {
                    "strings_as_keywords": {
                        "match_mapping_type": "string",
                        "mapping": {
                            "type": "keyword",
                        },
                    },
                },
            ],
            "dynamic": "strict",
            "properties": {
                # Search log key, search_id and timestamp.
                "search_id": {
                    "type": "keyword",
                },
                "created": {
                    "type": "date",
                },

                # Search log type, i.e., "query" or "click".
                "log_type": {
                    "type": "keyword",
                },

                # Query log type fields.
                "query": {
                    "type": "object",
                    "properties": {
                        "term": {
                            "type": "keyword",
                        },
                        "exact_terms": {
                            "type": "keyword",
                        },
                        "filters": {
                            "dynamic": True,
                            "type": "object",
                        },
                        "language_order": {
                            "type": "keyword",
                        },
                        "deb": {
                            "type": "boolean",
                        },
                        "intents": {
                            "type": "object",
                            "enabled": False,
                        },
                    },
                },
                "from": {
                    "type": "integer",
                },
                "size": {
                    "type": "integer",
                },
                "sort_by": {
                    "type": "keyword",
                },
                "query_result": {
                    "type": "object",
                    "enabled": False,
                },
                "error": {
                    "type": "object",
                    "enabled": False,
                },

                # Click log type fields.
                "mdb_uid": {
                    "type": "keyword",
                },
                "index": {
                    "type": "keyword",
                },
                "result_type": {
                    "type": "keyword",
                },
                "rank": {
                    "type": "integer",
                },

                # Log execition time for search components.
                "execution_time_log": {
                    "type": "nested",
                    "properties":
                    {
                        "operation": {"type": "keyword"},
                        "time": {"type": "integer"}
                    }
                },
                "is_debug": {
                    "type": "boolean",
                },
            },
        },
    },
}

def Resolve(lang, value):
  if isinstance(value, dict):
    l = [(k, Resolve(lang, v)) for (k, v) in value.iteritems()]
    return dict([(k, v) for k, v in l if v is not None])
  elif isinstance(value, list):
    return [x for x in [Resolve(lang, v) for v in value] if x is not None]
  elif callable(value):
    return value(lang)
  else:
    return value


for lang in LANG_GROUPS[ALL]:
  with open(os.path.join('.', 'data', 'es', 'mappings', 'results', 'results-%s.json' % lang), 'w') as f:
    json.dump(Resolve(lang, RESULTS_TEMPLATE), f, indent=4)
# Without languages
with open(os.path.join('.', 'data', 'es', 'mappings', 'search_logs.json'), 'w') as f:
  json.dump(Resolve('xx', SEARCH_LOGS_TEMPLATE), f, indent=4)
