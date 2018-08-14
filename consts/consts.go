package consts

/*
This is a modified version of the github.com/Bnei-Baruch/mdb/api/consts.go
 We take, manually, only what we need from there.
*/

const (
	// Collection Types
	CT_DAILY_LESSON       = "DAILY_LESSON"
	CT_SPECIAL_LESSON     = "SPECIAL_LESSON"
	CT_FRIENDS_GATHERINGS = "FRIENDS_GATHERINGS"
	CT_CONGRESS           = "CONGRESS"
	CT_VIDEO_PROGRAM      = "VIDEO_PROGRAM"
	CT_LECTURE_SERIES     = "LECTURE_SERIES"
	CT_VIRTUAL_LESSONS    = "VIRTUAL_LESSONS"
	CT_CHILDREN_LESSONS   = "CHILDREN_LESSONS"
	CT_WOMEN_LESSONS      = "WOMEN_LESSONS"
	CT_MEALS              = "MEALS"
	CT_HOLIDAY            = "HOLIDAY"
	CT_PICNIC             = "PICNIC"
	CT_UNITY_DAY          = "UNITY_DAY"
	CT_CLIPS              = "CLIPS"
	CT_ARTICLES           = "ARTICLES"
	CT_LESSONS_SERIES     = "LESSONS_SERIES"
	CT_SONGS              = "SONGS"
	CT_BOOKS              = "BOOKS"

	// Content Unit Types
	CT_LESSON_PART           = "LESSON_PART"
	CT_LECTURE               = "LECTURE"
	CT_VIRTUAL_LESSON        = "VIRTUAL_LESSON"
	CT_CHILDREN_LESSON       = "CHILDREN_LESSON"
	CT_WOMEN_LESSON          = "WOMEN_LESSON"
	CT_FRIENDS_GATHERING     = "FRIENDS_GATHERING"
	CT_MEAL                  = "MEAL"
	CT_VIDEO_PROGRAM_CHAPTER = "VIDEO_PROGRAM_CHAPTER"
	CT_FULL_LESSON           = "FULL_LESSON"
	CT_ARTICLE               = "ARTICLE"
	CT_EVENT_PART            = "EVENT_PART"
	CT_UNKNOWN               = "UNKNOWN"
	CT_CLIP                  = "CLIP"
	CT_TRAINING              = "TRAINING"
	CT_KITEI_MAKOR           = "KITEI_MAKOR"
	CT_PUBLICATION           = "PUBLICATION"
	CT_LELO_MIKUD            = "LELO_MIKUD"
	CT_SONG                  = "SONG"
	CT_BOOK                  = "BOOK"
	CT_BLOG_POST             = "BLOG_POST"

	// Content Role types
	CR_LECTURER = "LECTURER"

	// Persons patterns
	P_RAV = "rav"

	// Security levels
	SEC_PUBLIC    = int16(0)
	SEC_SENSITIVE = int16(1)
	SEC_PRIVATE   = int16(2)

	// Languages
	LANG_ENGLISH    = "en"
	LANG_HEBREW     = "he"
	LANG_RUSSIAN    = "ru"
	LANG_SPANISH    = "es"
	LANG_ITALIAN    = "it"
	LANG_GERMAN     = "de"
	LANG_DUTCH      = "nl"
	LANG_FRENCH     = "fr"
	LANG_PORTUGUESE = "pt"
	LANG_TURKISH    = "tr"
	LANG_POLISH     = "pl"
	LANG_ARABIC     = "ar"
	LANG_HUNGARIAN  = "hu"
	LANG_FINNISH    = "fi"
	LANG_LITHUANIAN = "lt"
	LANG_JAPANESE   = "ja"
	LANG_BULGARIAN  = "bg"
	LANG_GEORGIAN   = "ka"
	LANG_NORWEGIAN  = "no"
	LANG_SWEDISH    = "sv"
	LANG_CROATIAN   = "hr"
	LANG_CHINESE    = "zh"
	LANG_PERSIAN    = "fa"
	LANG_ROMANIAN   = "ro"
	LANG_HINDI      = "hi"
	LANG_UKRAINIAN  = "ua"
	LANG_MACEDONIAN = "mk"
	LANG_SLOVENIAN  = "sl"
	LANG_LATVIAN    = "lv"
	LANG_SLOVAK     = "sk"
	LANG_CZECH      = "cs"
	LANG_MULTI      = "zz"
	LANG_UNKNOWN    = "xx"
)

var ALL_KNOWN_LANGS = [...]string{
	LANG_ENGLISH, LANG_HEBREW, LANG_RUSSIAN, LANG_SPANISH, LANG_ITALIAN, LANG_GERMAN, LANG_DUTCH, LANG_FRENCH,
	LANG_PORTUGUESE, LANG_TURKISH, LANG_POLISH, LANG_ARABIC, LANG_HUNGARIAN, LANG_FINNISH, LANG_LITHUANIAN,
	LANG_JAPANESE, LANG_BULGARIAN, LANG_GEORGIAN, LANG_NORWEGIAN, LANG_SWEDISH, LANG_CROATIAN, LANG_CHINESE,
	LANG_PERSIAN, LANG_ROMANIAN, LANG_HINDI, LANG_MACEDONIAN, LANG_SLOVENIAN, LANG_LATVIAN, LANG_SLOVAK, LANG_CZECH,
	LANG_UKRAINIAN,
}

var I18N_LANG_ORDER = map[string][]string{
	"":              {LANG_ENGLISH},
	LANG_ENGLISH:    {LANG_ENGLISH},
	LANG_HEBREW:     {LANG_HEBREW, LANG_ENGLISH},
	LANG_RUSSIAN:    {LANG_RUSSIAN, LANG_ENGLISH},
	LANG_SPANISH:    {LANG_SPANISH, LANG_ENGLISH},
	LANG_ITALIAN:    {LANG_ITALIAN, LANG_ENGLISH},
	LANG_GERMAN:     {LANG_GERMAN, LANG_ENGLISH},
	LANG_DUTCH:      {LANG_DUTCH, LANG_ENGLISH},
	LANG_FRENCH:     {LANG_FRENCH, LANG_ENGLISH},
	LANG_PORTUGUESE: {LANG_PORTUGUESE, LANG_ENGLISH},
	LANG_TURKISH:    {LANG_TURKISH, LANG_ENGLISH},
	LANG_POLISH:     {LANG_POLISH, LANG_ENGLISH},
	LANG_ARABIC:     {LANG_ARABIC, LANG_ENGLISH},
	LANG_HUNGARIAN:  {LANG_HUNGARIAN, LANG_ENGLISH},
	LANG_FINNISH:    {LANG_FINNISH, LANG_ENGLISH},
	LANG_LITHUANIAN: {LANG_LITHUANIAN, LANG_RUSSIAN, LANG_ENGLISH},
	LANG_JAPANESE:   {LANG_JAPANESE, LANG_ENGLISH},
	LANG_BULGARIAN:  {LANG_BULGARIAN, LANG_ENGLISH},
	LANG_GEORGIAN:   {LANG_GEORGIAN, LANG_RUSSIAN, LANG_ENGLISH},
	LANG_NORWEGIAN:  {LANG_NORWEGIAN, LANG_ENGLISH},
	LANG_SWEDISH:    {LANG_SWEDISH, LANG_ENGLISH},
	LANG_CROATIAN:   {LANG_CROATIAN, LANG_ENGLISH},
	LANG_CHINESE:    {LANG_CHINESE, LANG_ENGLISH},
	LANG_PERSIAN:    {LANG_PERSIAN, LANG_ENGLISH},
	LANG_ROMANIAN:   {LANG_ROMANIAN, LANG_ENGLISH},
	LANG_HINDI:      {LANG_HINDI, LANG_ENGLISH},
	LANG_UKRAINIAN:  {LANG_UKRAINIAN, LANG_RUSSIAN, LANG_ENGLISH},
	LANG_MACEDONIAN: {LANG_MACEDONIAN, LANG_ENGLISH},
	LANG_SLOVENIAN:  {LANG_SLOVENIAN, LANG_ENGLISH},
	LANG_LATVIAN:    {LANG_LATVIAN, LANG_ENGLISH},
	LANG_SLOVAK:     {LANG_SLOVAK, LANG_ENGLISH},
	LANG_CZECH:      {LANG_CZECH, LANG_ENGLISH},
}

var SEARCH_LANG_ORDER = map[string][]string{
	"":           {LANG_ENGLISH},
	LANG_ENGLISH: {LANG_ENGLISH},
	LANG_HEBREW:  {LANG_HEBREW, LANG_ENGLISH},
	LANG_RUSSIAN: {LANG_RUSSIAN, LANG_ENGLISH},
	// Set English as first language to solve problem
	// of search like: "Yeshivat Haverim"
	// This is problematic, but should solve showing
	// Germal results for this query.
	LANG_SPANISH:    {LANG_ENGLISH, LANG_SPANISH},
	LANG_ITALIAN:    {LANG_ENGLISH, LANG_ITALIAN},
	LANG_GERMAN:     {LANG_ENGLISH, LANG_GERMAN},
	LANG_DUTCH:      {LANG_ENGLISH, LANG_DUTCH},
	LANG_FRENCH:     {LANG_ENGLISH, LANG_FRENCH},
	LANG_PORTUGUESE: {LANG_ENGLISH, LANG_PORTUGUESE},
	LANG_TURKISH:    {LANG_ENGLISH, LANG_TURKISH},
	LANG_POLISH:     {LANG_ENGLISH, LANG_POLISH},
	LANG_ARABIC:     {LANG_ENGLISH, LANG_ARABIC},
	LANG_HUNGARIAN:  {LANG_ENGLISH, LANG_HUNGARIAN},
	LANG_FINNISH:    {LANG_ENGLISH, LANG_FINNISH},
	LANG_LITHUANIAN: {LANG_ENGLISH, LANG_LITHUANIAN},
	LANG_JAPANESE:   {LANG_ENGLISH, LANG_JAPANESE},
	// Temporary disable until solved issue with Russian.
	LANG_BULGARIAN: {LANG_RUSSIAN, LANG_BULGARIAN, LANG_ENGLISH},
	LANG_GEORGIAN:  {LANG_ENGLISH, LANG_GEORGIAN},
	LANG_NORWEGIAN: {LANG_ENGLISH, LANG_NORWEGIAN},
	LANG_SWEDISH:   {LANG_ENGLISH, LANG_SWEDISH},
	LANG_CROATIAN:  {LANG_ENGLISH, LANG_CROATIAN},
	LANG_CHINESE:   {LANG_ENGLISH, LANG_CHINESE},
	LANG_PERSIAN:   {LANG_ENGLISH, LANG_PERSIAN},
	LANG_ROMANIAN:  {LANG_ENGLISH, LANG_ROMANIAN},
	LANG_HINDI:     {LANG_ENGLISH, LANG_HINDI},
	// Temporary disable until solved issue with Russian.
	LANG_UKRAINIAN: {LANG_RUSSIAN, LANG_UKRAINIAN, LANG_ENGLISH},
	// Temporary disable until solved issue with Russian.
	LANG_MACEDONIAN: {LANG_RUSSIAN, LANG_MACEDONIAN, LANG_ENGLISH},
	LANG_SLOVENIAN:  {LANG_ENGLISH, LANG_SLOVENIAN},
	LANG_LATVIAN:    {LANG_ENGLISH, LANG_LATVIAN},
	LANG_SLOVAK:     {LANG_ENGLISH, LANG_SLOVAK},
	LANG_CZECH:      {LANG_ENGLISH, LANG_CZECH},
}

var CODE2LANG = map[string]string{
	"ENG": LANG_ENGLISH,
	"HEB": LANG_HEBREW,
	"RUS": LANG_RUSSIAN,
	"SPA": LANG_SPANISH,
	"ITA": LANG_ITALIAN,
	"GER": LANG_GERMAN,
	"DUT": LANG_DUTCH,
	"FRE": LANG_FRENCH,
	"POR": LANG_PORTUGUESE,
	"TRK": LANG_TURKISH,
	"POL": LANG_POLISH,
	"ARB": LANG_ARABIC,
	"HUN": LANG_HUNGARIAN,
	"FIN": LANG_FINNISH,
	"LIT": LANG_LITHUANIAN,
	"JPN": LANG_JAPANESE,
	"BUL": LANG_BULGARIAN,
	"GEO": LANG_GEORGIAN,
	"NOR": LANG_NORWEGIAN,
	"SWE": LANG_SWEDISH,
	"HRV": LANG_CROATIAN,
	"CHN": LANG_CHINESE,
	"FAR": LANG_PERSIAN,
	"RON": LANG_ROMANIAN,
	"HIN": LANG_HINDI,
	"UKR": LANG_UKRAINIAN,
	"MKD": LANG_MACEDONIAN,
	"SLV": LANG_SLOVENIAN,
	"LAV": LANG_LATVIAN,
	"SLK": LANG_SLOVAK,
	"CZE": LANG_CZECH,
}

var LANG2CODE = map[string]string{
	LANG_ENGLISH:    "ENG",
	LANG_HEBREW:     "HEB",
	LANG_RUSSIAN:    "RUS",
	LANG_SPANISH:    "SPA",
	LANG_ITALIAN:    "ITA",
	LANG_GERMAN:     "GER",
	LANG_DUTCH:      "DUT",
	LANG_FRENCH:     "FRE",
	LANG_PORTUGUESE: "POR",
	LANG_TURKISH:    "TRK",
	LANG_POLISH:     "POL",
	LANG_ARABIC:     "ARB",
	LANG_HUNGARIAN:  "HUN",
	LANG_FINNISH:    "FIN",
	LANG_LITHUANIAN: "LIT",
	LANG_JAPANESE:   "JPN",
	LANG_BULGARIAN:  "BUL",
	LANG_GEORGIAN:   "GEO",
	LANG_NORWEGIAN:  "NOR",
	LANG_SWEDISH:    "SWE",
	LANG_CROATIAN:   "HRV",
	LANG_CHINESE:    "CHN",
	LANG_PERSIAN:    "FAR",
	LANG_ROMANIAN:   "RON",
	LANG_HINDI:      "HIN",
	LANG_UKRAINIAN:  "UKR",
	LANG_MACEDONIAN: "MKD",
	LANG_SLOVENIAN:  "SLV",
	LANG_LATVIAN:    "LAV",
	LANG_SLOVAK:     "SLK",
	LANG_CZECH:      "CZE",
}

// api

const (
	API_DEFAULT_PAGE_SIZE = 50
	API_MAX_PAGE_SIZE     = 1000
)

const (
	SORT_BY_RELEVANCE      = "relevance"
	SORT_BY_NEWER_TO_OLDER = "newertoolder"
	SORT_BY_OLDER_TO_NEWER = "oldertonewer"
	SORT_BY_SOURCE_FIRST   = "sourcefirst"
)

var SORT_BY_VALUES = map[string]bool{
	SORT_BY_RELEVANCE:      true,
	SORT_BY_NEWER_TO_OLDER: true,
	SORT_BY_OLDER_TO_NEWER: true,
}

const (
	FILTER_TAG                       = "tag"
	FILTER_START_DATE                = "start_date"
	FILTER_END_DATE                  = "end_date"
	FILTER_SOURCE                    = "source"
	FILTER_AUTHOR                    = "author"
	FILTER_UNITS_CONTENT_TYPES       = "units_content_types"
	FILTER_COLLECTIONS_CONTENT_TYPES = "collections_content_types"
	FILTER_SECTION_SOURCES           = "filter_section_sources"
)

// Use to identify and map request filters
// Maps request filter name to index field name.
var FILTERS = map[string]string{
	FILTER_TAG:                       "tags",
	FILTER_START_DATE:                "start_date",
	FILTER_END_DATE:                  "end_date",
	FILTER_SOURCE:                    "sources",
	FILTER_AUTHOR:                    "sources",
	FILTER_UNITS_CONTENT_TYPES:       "content_type",
	FILTER_COLLECTIONS_CONTENT_TYPES: "collection_content_type",
	FILTER_SECTION_SOURCES:           "filter_section_sources",
}

// ElasticSearch 'es'
const ES_CLASSIFICATIONS_INDEX = "classifications"
const ES_UNITS_INDEX = "units"
const ES_COLLECTIONS_INDEX = "collections"

const (
	MEDIA_MP4 = "video/mp4"
	MEDIA_MP3 = "audio/mpeg"
)

const CDN = "https://cdn.kabbalahmedia.info/"

const ES_SOURCES_INDEX = "sources"

// Search filter.
type SearchFilterType int

const (
	SEARCH_NO_FILTER              SearchFilterType = iota
	SEARCH_FILTER_ONLY_SOURCES    SearchFilterType = iota
	SEARCH_FILTER_WITHOUT_SOURCES SearchFilterType = iota
)

// Classicication index consts.
const (
	SOURCES_INDEX_TYPE         = "sources"
	TAGS_INDEX_TYPE            = "tags"
	SOURCE_CLASSIFICATION_TYPE = "source"
	TAG_CLASSIFICATION_TYPE    = "tag"
)

// Search intents
const (
	INTENT_TYPE_TAG    = "tag"
	INTENT_TYPE_SOURCE = "source"

	INTENT_INDEX_TAG         = "intent-tag"
	INTENT_INDEX_SOURCE      = "intent-source"
	INTENT_HIT_TYPE_PROGRAMS = "programs"
	INTENT_HIT_TYPE_LESSONS  = "lessons"
)

var INTENT_INDEX_BY_TYPE = map[string]string{
	INTENT_TYPE_TAG:    INTENT_INDEX_TAG,
	INTENT_TYPE_SOURCE: INTENT_INDEX_SOURCE,
}

var INTENT_HIT_TYPE_BY_CT = map[string]string{
	CT_LESSON_PART:           INTENT_HIT_TYPE_LESSONS,
	CT_VIDEO_PROGRAM_CHAPTER: INTENT_HIT_TYPE_PROGRAMS,
}
