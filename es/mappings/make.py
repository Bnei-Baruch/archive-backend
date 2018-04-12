#!/usr/bin/python2

# TODO: Rewrite this into go.
# This file creates all mapping for Elastic. It is required because some
# features are supported for some languages but not others. Also specific
# languages need specific treatment such as transliteration for cyrillic other
# specific tokenization for CJK.

# Mapping generated here requires the following Elasticsearch plugins:
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
StandardAnalyzer = {
  ENGLISH: "english",
  HEBREW: "hebrew",
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


UNITS_TEMPLATE = {
  "settings": {
    "index": {
      "analysis": {
        "analyzer": {
          "phonetic_analyzer": {
            "tokenizer": "standard",
            "filter": [
              "standard",
              "lowercase",
              lambda lang: IsCyrillic(lang, 'icu_transliterate'),
              "custom_phonetic",
            ],
          },
        },
        "filter": {
          "icu_transliterate": lambda lang: IsCyrillic(lang, {
            "type": "icu_transform",
            "id": "Any-Latin; NFD; [:Nonspacing Mark:] Remove; NFC",
          }),
          "custom_phonetic": {
            "type": "phonetic",
            "encoder": "beider_morse",
            "replace": True,
            "languageset": BeiderMorseLanguageset,
          },
        },
      },
    },
  },
  "mappings": {
    "content_units": {
      "_all": {
        "enabled": False,
      },
      "properties": {
        "mdb_uid": {
          "type": "keyword",
        },
        "typed_uids": {
          "type": "keyword",
        },
        "name": {
          "type": "text",
          "analyzer": "phonetic_analyzer",
          "fields": {
            "analyzed": {
              "type": "text",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            },
            "length": {
              "type": "token_count",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            },
          },
        },
        "description": {
          "type": "text",
          "analyzer": "phonetic_analyzer",
          "fields": {
            "analyzed": {
              "type": "text",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            },
            "length": {
              "type": "token_count",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            },
          },
        },
        "content_type": {
          "type": "keyword",
        },
        "collections_content_types": {
          "type": "keyword",
        },
        "effective_date": {
          "type": "date",
          "format": "strict_date",
        },
        "duration": {
          "type": "short",
          "index": False,
        },
        "original_language": {
          "type": "keyword",
          "index": False,
        },
        "translations": {
          "type": "keyword",
          "index": False,
        },
        "tags": {
          "type": "keyword",
        },
        "sources": {
          "type": "keyword",
        },
        "persons": {
          "type": "keyword",
          "index": False,
        },
        "transcript": {
          "type": "text",
          "analyzer": "phonetic_analyzer",
          "fields": {
            "analyzed": {
              "type": "text",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            },
            "length": {
              "type": "token_count",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            },
          },
        },
      },
    },
  },
}

CLASSIFICATIONS_TEMPLATE = {
  "mappings": {
    "tags": {
      "_all": {
        "enabled": False,
      },
      "properties": {
        "mdb_uid": {
          "type": "keyword",
        },
        "classification_type": {
          "type": "keyword",
        },
        "name": {
          "type": "text",
          "analyzer": lambda lang: StandardAnalyzer[lang],
        },
        "name_suggest": {
          "type": "completion",
          "contexts": [
            {
              "name": "classification",
              "type": "category",
              "path": "classification_type",
            },
          ],
        },
      },
    },
    "sources": {
      "_all": {
        "enabled": False,
      },
      "properties": {
        "mdb_uid": {
          "type": "keyword",
        },
        "classification_type": {
          "type": "keyword",
        },
        "name": {
          "type": "text",
          "analyzer": lambda lang: StandardAnalyzer[lang],
        },
        "name_suggest": {
          "type": "completion",
          "contexts": [
            {
              "name": "classification",
              "type": "category",
              "path": "classification_type",
            },
          ],
        },
        "description": {
          "type": "text",
          "analyzer": lambda lang: StandardAnalyzer[lang],
        },
        "description_suggest": {
          "type": "completion",
          "contexts": [
            {
              "name": "classification",
              "type": "category",
              "path": "classification_type"
            },
          ],
        },
      },
    },
  },
}

COLLECTIONS_TEMPLATE = {
  "settings": {
    "index": {
      "analysis": {
        "analyzer": {
          "phonetic_analyzer": {
            "tokenizer": "standard",
            "filter": [
              "standard",
              "lowercase",
              lambda lang: IsCyrillic(lang, 'icu_transliterate'),
              "custom_phonetic",
            ],
          },
        },
        "filter": {
          "icu_transliterate": lambda lang: IsCyrillic(lang, {
            "type": "icu_transform",
            "id": "Any-Latin; NFD; [:Nonspacing Mark:] Remove; NFC",
          }),
          "custom_phonetic": {
            "type": "phonetic",
            "encoder": "beider_morse",
            "replace": True,
            "languageset": BeiderMorseLanguageset,
          },
        },
      },
    },
  },
  "mappings": {
    "collections": {
      "_all": {
        "enabled": False,
      },
      "properties": {
        "mdb_uid": {
          "type": "keyword",
        },
        "typed_uids": {
          "type": "keyword",
        },
        "name": {
          "type": "text",
          "analyzer": "phonetic_analyzer",
          "fields": {
            "analyzed": {
              "type": "text",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            },
            "length": {
              "type": "token_count",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            },
          },
        },
        "description": {
          "type": "text",
          "analyzer": "phonetic_analyzer",
          "fields": {
            "analyzed": {
              "type": "text",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            },
            "length": {
              "type": "token_count",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            },
          },
        },
        "content_type": {
          "type": "keyword",
        },
        "content_units_content_types": {
          "type": "keyword",
        },
        "effective_date": {
          "type": "date",
          "format": "strict_date",
        },
        "original_language": {
          "type": "keyword",
          "index": False,
        },
      },
    },
  },
}

SOURCES_TEMPLATE = {
  "settings": {
    "index": {
      "analysis": {
        "analyzer": {
          "phonetic_analyzer": {
            "tokenizer": "standard",
            "filter": [
              "standard",
              "lowercase",
              lambda lang: IsCyrillic(lang, 'icu_transliterate'),
              "custom_phonetic",
            ],
          },
        },
        "filter": {
          "icu_transliterate": lambda lang: IsCyrillic(lang, {
            "type": "icu_transform",
            "id": "Any-Latin; NFD; [:Nonspacing Mark:] Remove; NFC",
          }),
          "custom_phonetic": {
            "type": "phonetic",
            "encoder": "beider_morse",
            "replace": True,
            "languageset": BeiderMorseLanguageset,
          },
        },
      },
    },
  },
  "mappings": {
    "sources": {
      "_all": {
        "enabled": False,
      },
      "properties": {
        "mdb_uid": {
          "type": "keyword",
        },
        "name": {
          "type": "text",
          "analyzer": "phonetic_analyzer",
          "fields": {
            "analyzed": {
              "type": "text",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            },
          },
        },
        "description": {
          "type": "text",
          "analyzer": "phonetic_analyzer",
          "fields": {
            "analyzed": {
              "type": "text",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            },
          },
        },
        "authors": {
          "type": "text",
          "analyzer": "phonetic_analyzer",
          "fields": {
            "analyzed": {
              "type": "text",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            }
          },
        },
        "content": {
          "type": "text",
          "analyzer": "phonetic_analyzer",
          "fields": {
            "analyzed": {
              "type": "text",
              "analyzer": lambda lang: StandardAnalyzer[lang],
            }
          },
        },
        "sources": {
          "type": "keyword",
        }
      },
    },
  },
}


SEARCH_LOGS_TEMPLATE = {
    "mappings": {
        "search_logs": {
            "properties": {
                "search_id": {
                    "type": "keyword",
                },
                "created": {
                    "type": "date",
                },
                "query": {
                    "type": "object",
                },
                "from": {
                    "type": "integer",
                },
                "size": {
                    "type": "integer",
                },
                "results": {
                    "type": "object",
                },
                "sort_by": {
                    "type": "keyword",
                },
                "error": {
                    "type": "object",
                },
            },
        },
        "search_clicks": {
            "properties": {
                "created": {
                    "type": "date",
                },
                "mdb_uid": {
                    "type": "keyword",
                },
                "index": {
                    "type": "keyword",
                },
                "type": {
                    "type": "keyword",
                },
                "rank": {
                    "type": "integer",
                },
                "search_id": {
                    "type": "keyword",
                },
            },
            "_parent": {
                "type": "search_logs",
            }
        }
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
  with open(os.path.join('.', 'data', 'es', 'mappings', 'units', 'units-%s.json' % lang), 'w') as f:
    json.dump(Resolve(lang, UNITS_TEMPLATE), f, indent=4)
  with open(os.path.join('.', 'data', 'es', 'mappings', 'classifications', 'classifications-%s.json' % lang), 'w') as f:
    json.dump(Resolve(lang, CLASSIFICATIONS_TEMPLATE), f, indent=4)
  with open(os.path.join('.', 'data', 'es', 'mappings', 'collections', 'collections-%s.json' % lang), 'w') as f:
    json.dump(Resolve(lang, COLLECTIONS_TEMPLATE), f, indent=4)
  with open(os.path.join('.', 'data', 'es', 'mappings', 'sources', 'sources-%s.json' % lang), 'w') as f:
    json.dump(Resolve(lang, SOURCES_TEMPLATE), f, indent=4)
# Without languages
with open(os.path.join('.', 'data', 'es', 'mappings', 'search_logs.json'), 'w') as f:
  json.dump(Resolve('xx', SEARCH_LOGS_TEMPLATE), f, indent=4)
