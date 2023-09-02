package consts

/*
This is a modified version of the github.com/Bnei-Baruch/mdb/api/consts.go
 We take, manually, only what we need from there.
*/

const (
	// Collection Types
	CT_ARTICLES           = "ARTICLES"
	CT_BOOKS              = "BOOKS"
	CT_CHILDREN_LESSONS   = "CHILDREN_LESSONS"
	CT_CLIPS              = "CLIPS"
	CT_CONGRESS           = "CONGRESS"
	CT_DAILY_LESSON       = "DAILY_LESSON"
	CT_FRIENDS_GATHERINGS = "FRIENDS_GATHERINGS"
	CT_HOLIDAY            = "HOLIDAY"
	CT_LECTURE_SERIES     = "LECTURE_SERIES"
	CT_LESSONS_SERIES     = "LESSONS_SERIES"
	CT_MEALS              = "MEALS"
	CT_PICNIC             = "PICNIC"
	CT_SONGS              = "SONGS"
	CT_SPECIAL_LESSON     = "SPECIAL_LESSON"
	CT_UNITY_DAY          = "UNITY_DAY"
	CT_VIDEO_PROGRAM      = "VIDEO_PROGRAM"
	CT_VIRTUAL_LESSONS    = "VIRTUAL_LESSONS"
	CT_WOMEN_LESSONS      = "WOMEN_LESSONS"

	// Content Unit Types
	CT_ARTICLE               = "ARTICLE"
	CT_BLOG_POST             = "BLOG_POST"
	CT_BOOK                  = "BOOK"
	CT_CHILDREN_LESSON       = "CHILDREN_LESSON"
	CT_CLIP                  = "CLIP"
	CT_EVENT_PART            = "EVENT_PART"
	CT_FRIENDS_GATHERING     = "FRIENDS_GATHERING"
	CT_FULL_LESSON           = "FULL_LESSON"
	CT_KITEI_MAKOR           = "KITEI_MAKOR"
	CT_LECTURE               = "LECTURE"
	CT_LELO_MIKUD            = "LELO_MIKUD"
	CT_LESSON_PART           = "LESSON_PART"
	CT_MEAL                  = "MEAL"
	CT_PUBLICATION           = "PUBLICATION"
	CT_RESEARCH_MATERIAL     = "RESEARCH_MATERIAL"
	CT_KTAIM_NIVCHARIM       = "KTAIM_NIVCHARIM"
	CT_SONG                  = "SONG"
	CT_TRAINING              = "TRAINING"
	CT_UNKNOWN               = "UNKNOWN"
	CT_VIDEO_PROGRAM_CHAPTER = "VIDEO_PROGRAM_CHAPTER"
	CT_VIRTUAL_LESSON        = "VIRTUAL_LESSON"
	CT_WOMEN_LESSON          = "WOMEN_LESSON"
	CT_SOURCE                = "SOURCE"
	CT_LIKUTIM               = "LIKUTIM"

	// Content types for additional Elastic results
	SCT_TWEET = "R_TWEET"

	// Content Role types
	CR_LECTURER = "LECTURER"

	// Persons patterns
	P_RAV    = "rav"
	P_RABASH = "rb"

	// Source types
	SRC_TYPE_COLLECTION = 1
	SRC_TYPE_BOOK       = 2
	SRC_TYPE_VOLUME     = 3
	SRC_TYPE_PART       = 4
	SRC_TYPE_PARASHA    = 5
	SRC_TYPE_CHAPTER    = 6
	SRC_TYPE_ARTICLE    = 7
	SRC_TYPE_TITLE      = 8
	SRC_TYPE_LETTER     = 9
	SRC_TYPE_ITEM       = 10

	// Hit Types
	HT_LESSONS_SERIES = "lessons-series"

	// Security levels
	SEC_PUBLIC    = int16(0)
	SEC_SENSITIVE = int16(1)
	SEC_PRIVATE   = int16(2)

	// Approve state levels
	APR_NONE     = int16(0)
	APR_APPROVED = int16(1)
	APR_DECLINED = int16(2)

	// Weight of 'sources' and 'collections' autocomplete results (assigned at index time)
	ES_SOURCES_SUGGEST_DEFAULT_WEIGHT     = 50
	ES_COLLECTIONS_SUGGEST_DEFAULT_WEIGHT = 40

	ES_GRAMMAR_SUGGEST_DEFAULT_WEIGHT = 100

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
	LANG_AMHARIC    = "am"
	LANG_INDONESIAN = "id"
	LANG_ARMENIAN   = "hy"
	LANG_MULTI      = "zz"
	LANG_UNKNOWN    = "xx"

	//suggest constant terms patterns
	TERMS_PATTERN_DIGITS = `(?:^|\s|\(|\/)(\d+)(\b)`

	DEFAULT_UI_LANGUAGE = LANG_ENGLISH
)

var COLLECTIONS_CONTENT_TYPES = []string{
	CT_ARTICLES,
	CT_BOOKS,
	CT_CHILDREN_LESSONS,
	CT_CLIPS,
	CT_CONGRESS,
	CT_DAILY_LESSON,
	CT_FRIENDS_GATHERINGS,
	CT_HOLIDAY,
	CT_LECTURE_SERIES,
	CT_LESSONS_SERIES,
	CT_MEALS,
	CT_PICNIC,
	CT_SONGS,
	CT_SPECIAL_LESSON,
	CT_UNITY_DAY,
	CT_VIDEO_PROGRAM,
	CT_VIRTUAL_LESSONS,
	CT_WOMEN_LESSONS,
}

var SECTION_CT_TYPES = [...]string{
	CT_ARTICLE,
	CT_BLOG_POST,
	CT_CLIP,
	CT_EVENT_PART,
	CT_FRIENDS_GATHERING,
	CT_LECTURE,
	CT_LESSONS_SERIES,
	CT_LESSON_PART,
	CT_MEAL,
	CT_SOURCE,
	CT_VIDEO_PROGRAM_CHAPTER,
	CT_VIRTUAL_LESSON,
	CT_WOMEN_LESSON,
	SCT_TWEET,
}

var CT_NOT_FOR_DISPLAY = [...]string{CT_BOOK, CT_CHILDREN_LESSON, CT_FULL_LESSON, CT_KITEI_MAKOR, CT_LELO_MIKUD, CT_RESEARCH_MATERIAL, CT_UNKNOWN}

var ALL_KNOWN_LANGS = [...]string{
	LANG_ENGLISH, LANG_HEBREW, LANG_RUSSIAN, LANG_SPANISH, LANG_ITALIAN, LANG_GERMAN, LANG_DUTCH, LANG_FRENCH,
	LANG_PORTUGUESE, LANG_TURKISH, LANG_POLISH, LANG_ARABIC, LANG_HUNGARIAN, LANG_FINNISH, LANG_LITHUANIAN,
	LANG_JAPANESE, LANG_BULGARIAN, LANG_GEORGIAN, LANG_NORWEGIAN, LANG_SWEDISH, LANG_CROATIAN, LANG_CHINESE,
	LANG_PERSIAN, LANG_ROMANIAN, LANG_HINDI, LANG_MACEDONIAN, LANG_SLOVENIAN, LANG_LATVIAN, LANG_SLOVAK, LANG_CZECH,
	LANG_UKRAINIAN, LANG_AMHARIC, LANG_INDONESIAN, LANG_ARMENIAN,
}

var ALL_SRC_TYPES = []int64{
	SRC_TYPE_COLLECTION, SRC_TYPE_BOOK, SRC_TYPE_VOLUME, SRC_TYPE_PART, SRC_TYPE_PARASHA, SRC_TYPE_CHAPTER, SRC_TYPE_ARTICLE, SRC_TYPE_TITLE, SRC_TYPE_LETTER, SRC_TYPE_ITEM,
}

var SRC_TYPES_FOR_TITLE_DESCRIPTION_CONCAT = map[int64]bool{
	SRC_TYPE_VOLUME: true,
	SRC_TYPE_PART:   true,
}

var ANALYZERS = map[string]string{
	LANG_AMHARIC:    "standard",
	LANG_ARABIC:     "arabic",
	LANG_BULGARIAN:  "bulgarian",
	LANG_CZECH:      "czech",
	LANG_GERMAN:     "german",
	LANG_ENGLISH:    "english_synonym",
	LANG_SPANISH:    "spanish_synonym",
	LANG_PERSIAN:    "persian",
	LANG_FINNISH:    "finnish",
	LANG_FRENCH:     "french",
	LANG_HEBREW:     "hebrew_synonym",
	LANG_HINDI:      "hindi",
	LANG_CROATIAN:   "standard",
	LANG_HUNGARIAN:  "hungarian",
	LANG_ITALIAN:    "italian",
	LANG_JAPANESE:   "cjk",
	LANG_GEORGIAN:   "standard",
	LANG_LITHUANIAN: "lithuanian",
	LANG_LATVIAN:    "latvian",
	LANG_MACEDONIAN: "standard",
	LANG_DUTCH:      "dutch",
	LANG_NORWEGIAN:  "norwegian",
	LANG_POLISH:     "standard",
	LANG_PORTUGUESE: "portuguese",
	LANG_ROMANIAN:   "romanian",
	LANG_RUSSIAN:    "russian_synonym",
	LANG_SLOVAK:     "standard",
	LANG_SLOVENIAN:  "standard",
	LANG_SWEDISH:    "swedish",
	LANG_TURKISH:    "turkish",
	LANG_UKRAINIAN:  "standard",
	LANG_CHINESE:    "cjk",
	LANG_INDONESIAN: "indonesian",
	LANG_ARMENIAN:   "armenian",
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
	LANG_AMHARIC:    {LANG_AMHARIC, LANG_ENGLISH},
	LANG_INDONESIAN: {LANG_INDONESIAN, LANG_ENGLISH},
	LANG_ARMENIAN:   {LANG_ARMENIAN, LANG_ENGLISH},
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
	LANG_AMHARIC:    {LANG_ENGLISH, LANG_ENGLISH},
	LANG_INDONESIAN: {LANG_ENGLISH, LANG_INDONESIAN},
	LANG_ARMENIAN:   {LANG_ENGLISH, LANG_ARMENIAN},
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
	"ARA": LANG_ARABIC,
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
	"PER": LANG_PERSIAN,
	"RON": LANG_ROMANIAN,
	"HIN": LANG_HINDI,
	"UKR": LANG_UKRAINIAN,
	"MKD": LANG_MACEDONIAN,
	"SLV": LANG_SLOVENIAN,
	"LAV": LANG_LATVIAN,
	"SLK": LANG_SLOVAK,
	"CZE": LANG_CZECH,
	"AMH": LANG_AMHARIC,
	"IND": LANG_INDONESIAN,
	"ARM": LANG_ARMENIAN,
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
	LANG_ARABIC:     "ARA",
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
	LANG_PERSIAN:    "PER",
	LANG_ROMANIAN:   "RON",
	LANG_HINDI:      "HIN",
	LANG_UKRAINIAN:  "UKR",
	LANG_MACEDONIAN: "MKD",
	LANG_SLOVENIAN:  "SLV",
	LANG_LATVIAN:    "LAV",
	LANG_SLOVAK:     "SLK",
	LANG_CZECH:      "CZE",
	LANG_AMHARIC:    "AMH",
	LANG_INDONESIAN: "IND",
	LANG_ARMENIAN:   "ARM",
}

// api

const (
	INTENTS_SEARCH_DEFAULT_COUNT              = 10
	INTENTS_SEARCH_BY_FILTER_GRAMMAR_COUNT    = 2
	TWEETS_SEARCH_COUNT                       = 20
	INTENTS_MIN_UNITS                         = 3 // Min. units from Intent Engine (not grammar) to display the carousel
	MAX_CLASSIFICATION_INTENTS                = 3
	API_DEFAULT_PAGE_SIZE                     = 50
	API_MAX_PAGE_SIZE                         = 1000
	MIN_RESULTS_SCORE_TO_IGNOGRE_TYPO_SUGGEST = 100
	// We display the carousels generated from Grammar Engine even if we have just one unit inside
	// since the regular results might not return results at all based on the given query.
	CLASSIFICATION_FROM_GRAMMAR_INTENTS_MIN_UNITS = 1
	// Consider making a carusele and not limiting.
	MAX_MATCHES_PER_GRAMMAR_INTENT                  = 3
	FILTER_GRAMMAR_INCREMENT_FOR_MATCH_TO_FULL_TERM = 200
	CONTENT_TYPE_INTENTS_BOOST                      = 8.0 // For priority between several filter intent types
	SCORE_INCREMENT_FOR_SEARCH_WITHOUT_TERM_RESULTS = 200.0
	MAX_GRAMMAR_INTENTS_FOR_FILTER_SEARCH           = 4
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
	// Frontend filter names.
	FILTER_TAG                      = "tag"
	FILTER_START_DATE               = "start_date"
	FILTER_END_DATE                 = "end_date"
	FILTER_SOURCE                   = "source"
	FILTER_AUTHOR                   = "author"
	FILTER_CONTENT_TYPE             = "content_type"
	FILTER_COLLECTIONS_CONTENT_TYPE = "collections_content_type"
	FILTER_MEDIA_LANGUAGE           = "media_language"
	FILTER_ORIGINAL_LANGUAGE        = "original_language"
	FILTER_PERSON                   = "person"

	// Used to name facets aggregation.
	AGG_FILTER_DATES         = "dates"
	DATE_FILTER_TODAY        = "TODAY"
	DATE_FILTER_YESTERDAY    = "YESTERDAY"
	DATE_FILTER_LAST_7_DAYS  = "LAST_7_DAYS"
	DATE_FILTER_LAST_30_DAYS = "LAST_30_DAYS"

	FILTER_COLLECTION = "collection" //  Internally used by grammar. Not available in frontend.
)

var ALL_FILTERS = []string{
	FILTER_TAG,
	FILTER_START_DATE,
	FILTER_END_DATE,
	FILTER_SOURCE,
	FILTER_AUTHOR,
	FILTER_CONTENT_TYPE,
	FILTER_MEDIA_LANGUAGE,
	FILTER_ORIGINAL_LANGUAGE,
	FILTER_PERSON,
}

// ElasticSearch 'es'
const ES_RESULTS_INDEX = "results"

// Result type
const ES_RESULT_TYPE = "result_type"
const ES_RESULT_TYPE_UNITS = "units"
const ES_RESULT_TYPE_SOURCES = "sources"
const ES_RESULT_TYPE_COLLECTIONS = "collections"
const ES_RESULT_TYPE_TAGS = "tags"
const ES_RESULT_TYPE_BLOG_POSTS = "posts"
const ES_RESULT_TYPE_TWEETS = "tweets"

// Result of many tweets in one hit
const SEARCH_RESULT_TWEETS_MANY = "tweets_many"
const SEARCH_RESULT_LESSONS_SERIES_BY_SOURCE = "lessons_series_by_source"
const SEARCH_RESULT_LESSONS_SERIES_BY_TAG = "lessons_series_by_tag"

// Additional result types for mobile client
const SEARCH_RESULT_PROGRAMS_BY_SOURCE = "programs_by_source"
const SEARCH_RESULT_PROGRAMS_BY_TAG = "programs_by_tag"
const SEARCH_RESULT_LESSONS_BY_SOURCE = "lessons_by_source"
const SEARCH_RESULT_LESSONS_BY_TAG = "lessons_by_tag"

// Typed UIDs and Filter
const ES_UID_TYPE_CONTENT_UNIT = "content_unit"
const ES_UID_TYPE_FILE = "file"
const ES_UID_TYPE_TAG = "tag"
const ES_UID_TYPE_TAGS = "tags"
const ES_UID_TYPE_COLLECTION = "collection"
const ES_UID_TYPE_SOURCE = "source"
const ES_UID_TYPE_TWEET = "tweet"
const ES_UID_TYPE_BLOG_POST = "blog_post"

// ES_RESULT_TYPE_TWEETS is not part of the array since it's searched in parallel to other results search
var ES_SEARCH_RESULT_TYPES = []string{
	ES_RESULT_TYPE_UNITS,
	ES_RESULT_TYPE_SOURCES,
	ES_RESULT_TYPE_COLLECTIONS,
	ES_RESULT_TYPE_BLOG_POSTS,
}

var ES_MOBILE_SEARCH_RESULT_TYPES = []string{
	ES_RESULT_TYPE_UNITS,
	ES_RESULT_TYPE_SOURCES,
	ES_RESULT_TYPE_COLLECTIONS,
}

var ES_ALL_RESULT_TYPES = []string{
	ES_RESULT_TYPE_UNITS,
	ES_RESULT_TYPE_TAGS,
	ES_RESULT_TYPE_SOURCES,
	ES_RESULT_TYPE_COLLECTIONS,
	ES_RESULT_TYPE_BLOG_POSTS,
	ES_RESULT_TYPE_TWEETS,
}

const (
	MEDIA_MP4  = "video/mp4"
	MEDIA_MP3  = "audio/mpeg"
	MEDIA_MP3a = "audio/mpeg"
	MEDIA_MP3b = "audio/mp3"
)

const CDN = "https://cdn.kabbalahmedia.info/"

// TokensCache LRU cache size
const TOKEN_CACHE_SIZE = 10000

// Search filter.
type SearchFilterType int

const (
	SEARCH_NO_FILTER              SearchFilterType = iota
	SEARCH_FILTER_ONLY_SOURCES    SearchFilterType = iota
	SEARCH_FILTER_WITHOUT_SOURCES SearchFilterType = iota
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

var ES_INTENT_SUPPORTED_FILTERS = map[string]bool{
	FILTER_CONTENT_TYPE: true,
	FILTER_TAG:          true,
	FILTER_SOURCE:       true,
}

// If these filters present, we automatically add some search results when the search term is identical to source name.
var AUTO_INTENTS_BY_SOURCE_NAME_SUPPORTED_FILTERS = map[string]bool{
	FILTER_CONTENT_TYPE: true,
	// FILTER_SECTION_SOURCES: true,
}

var ES_INTENT_SUPPORTED_CONTENT_TYPES = map[string]bool{
	CT_LESSON_PART:           true,
	CT_LECTURE:               true,
	CT_VIRTUAL_LESSON:        true,
	CT_CHILDREN_LESSON:       true,
	CT_WOMEN_LESSON:          true,
	CT_VIDEO_PROGRAM_CHAPTER: true,
	CT_FULL_LESSON:           true,
	CT_CLIP:                  true,
}

type IntentSearchOptions struct {
	SearchTags    bool
	SearchSources bool
	ContentTypes  []string
}

var INTENT_OPTIONS_BY_GRAMMAR_CT_VARIABLES = map[string]IntentSearchOptions{
	VAR_CT_PROGRAMS: IntentSearchOptions{
		SearchSources: true,
		SearchTags:    true,
		ContentTypes:  []string{CT_VIDEO_PROGRAM_CHAPTER},
	},
	VAR_CT_ARTICLES: IntentSearchOptions{
		SearchSources: true,
		SearchTags:    false,
		ContentTypes:  []string{CT_VIDEO_PROGRAM_CHAPTER, CT_LESSON_PART},
	},
	VAR_CT_LESSONS: IntentSearchOptions{
		SearchSources: true,
		SearchTags:    true,
		ContentTypes:  []string{CT_LESSON_PART},
	},
	VAR_CT_BOOK_TITLES: IntentSearchOptions{
		SearchSources: true,
		SearchTags:    false,
		ContentTypes:  []string{CT_VIDEO_PROGRAM_CHAPTER, CT_LESSON_PART},
	},
}

// Fake index for intents.
var INTENT_INDEX_BY_TYPE = map[string]string{
	INTENT_TYPE_TAG:    INTENT_INDEX_TAG,
	INTENT_TYPE_SOURCE: INTENT_INDEX_SOURCE,
}

var RESULT_TYPE_BY_INDEX_TYPE = map[string]string{
	INTENT_TYPE_TAG:    ES_RESULT_TYPE_TAGS,
	INTENT_TYPE_SOURCE: ES_RESULT_TYPE_SOURCES,
}

var INTENT_HIT_TYPE_BY_CT = map[string]string{
	CT_LESSON_PART:           INTENT_HIT_TYPE_LESSONS,
	CT_VIDEO_PROGRAM_CHAPTER: INTENT_HIT_TYPE_PROGRAMS,
}

const (
	GRAMMAR_INDEX = "grammar"

	GRAMMAR_TYPE_FILTER              = "filter"
	GRAMMAR_TYPE_FILTER_WITHOUT_TERM = "filter_without_term"
	GRAMMAR_TYPE_LANDING_PAGE        = "landing-page"
	GRAMMAR_TYPE_CLASSIFICATION      = "classification"

	GRAMMAR_INTENT_FILTER_BY_CONTENT_TYPE         = "by_content_type"
	GRAMMAR_INTENT_FILTER_BY_SOURCE               = "by_source"
	GRAMMAR_INTENT_FILTER_BY_PROGRAM              = "by_program"
	GRAMMAR_INTENT_FILTER_BY_PROGRAM_WITHOUT_TERM = "by_program_without_term"
	GRAMMAR_INTENT_SOURCE_POSITION_WITHOUT_TERM   = "source_position_without_term"
	GRAMMAR_INTENT_PROGRAM_POSITION_WITHOUT_TERM  = "program_position_without_term"

	GRAMMAR_LP_SINGLE_COLLECTION = "grammar_landing_page_single_collection_from_sql"
	GRAMMAR_GENERATED_CU_HIT     = "grammar_generated_content_unit_hit"
	GRAMMAR_GENERATED_SOURCE_HIT = "grammar_generated_source_hit"

	GRAMMAR_INTENT_LANDING_PAGE_LESSONS            = "lessons"
	GRAMMAR_INTENT_LANDING_PAGE_VIRTUAL_LESSONS    = "virtual_lessons"
	GRAMMAR_INTENT_LANDING_PAGE_LECTURES           = "lectures"
	GRAMMAR_INTENT_LANDING_PAGE_WOMEN_LESSONS      = "women_lessons"
	GRAMMAR_INTENT_LANDING_PAGE_RABASH_LESSONS     = "rabash_lessons"
	GRAMMAR_INTENT_LANDING_PAGE_LESSON_SERIES      = "lesson_series"
	GRAMMAR_INTENT_LANDING_PAGE_PRORGRAMS          = "programs"
	GRAMMAR_INTENT_LANDING_PAGE_CLIPS              = "clips"
	GRAMMAR_INTENT_LANDING_PAGE_LIBRARY            = "library"
	GRAMMAR_INTENT_LANDING_PAGE_GROUP_ARTICLES     = "group_articles"
	GRAMMAR_INTENT_LANDING_PAGE_CONVENTIONS        = "conventions"
	GRAMMAR_INTENT_LANDING_PAGE_HOLIDAYS           = "holidays"
	GRAMMAR_INTENT_LANDING_PAGE_UNITY_DAYS         = "unity_days"
	GRAMMAR_INTENT_LANDING_PAGE_FRIENDS_GATHERINGS = "friends_gatherings"
	GRAMMAR_INTENT_LANDING_PAGE_MEALS              = "meals"
	GRAMMAR_INTENT_LANDING_PAGE_TOPICS             = "topics"
	GRAMMAR_INTENT_LANDING_PAGE_BLOG               = "blog"
	GRAMMAR_INTENT_LANDING_PAGE_TWITTER            = "twitter"
	GRAMMAR_INTENT_LANDING_PAGE_ARTICLES           = "articles"
	GRAMMAR_INTENT_LANDING_PAGE_DOWNLOADS          = "downloads"
	GRAMMAR_INTENT_LANDING_PAGE_HELP               = "help"
)

// Map from intent to filters, i.e., filter name to list of values.
var GRAMMAR_INTENTS_TO_FILTER_VALUES = map[string]map[string][]string{
	// Landing pages
	GRAMMAR_INTENT_LANDING_PAGE_LESSONS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_LESSON_PART, CT_FULL_LESSON, CT_DAILY_LESSON},
	},
	GRAMMAR_INTENT_LANDING_PAGE_VIRTUAL_LESSONS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_VIRTUAL_LESSON, CT_VIRTUAL_LESSONS},
	},
	GRAMMAR_INTENT_LANDING_PAGE_LECTURES: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_LECTURE, CT_LECTURE_SERIES},
	},
	GRAMMAR_INTENT_LANDING_PAGE_WOMEN_LESSONS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_WOMEN_LESSON, CT_WOMEN_LESSONS},
	},
	GRAMMAR_INTENT_LANDING_PAGE_RABASH_LESSONS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_LESSON_PART, CT_DAILY_LESSON},
	},
	GRAMMAR_INTENT_LANDING_PAGE_LESSON_SERIES: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_LESSONS_SERIES},
	},
	GRAMMAR_INTENT_LANDING_PAGE_PRORGRAMS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_VIDEO_PROGRAM_CHAPTER, CT_VIDEO_PROGRAM},
	},
	GRAMMAR_INTENT_LANDING_PAGE_CLIPS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_CLIP},
	},
	GRAMMAR_INTENT_LANDING_PAGE_LIBRARY: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_SOURCE},
	},
	GRAMMAR_INTENT_LANDING_PAGE_GROUP_ARTICLES: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_SOURCE},
	},
	GRAMMAR_INTENT_LANDING_PAGE_CONVENTIONS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_EVENT_PART, CT_CONGRESS},
	},
	GRAMMAR_INTENT_LANDING_PAGE_HOLIDAYS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_EVENT_PART, CT_HOLIDAY},
	},
	GRAMMAR_INTENT_LANDING_PAGE_UNITY_DAYS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_EVENT_PART, CT_UNITY_DAY},
	},
	GRAMMAR_INTENT_LANDING_PAGE_FRIENDS_GATHERINGS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_FRIENDS_GATHERING, CT_FRIENDS_GATHERINGS},
	},
	GRAMMAR_INTENT_LANDING_PAGE_MEALS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_MEAL, CT_MEALS},
	},
	GRAMMAR_INTENT_LANDING_PAGE_TOPICS: nil,
	GRAMMAR_INTENT_LANDING_PAGE_BLOG: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_BLOG_POST, CT_ARTICLES},
	},
	GRAMMAR_INTENT_LANDING_PAGE_TWITTER: map[string][]string{
		FILTER_CONTENT_TYPE: []string{SCT_TWEET, CT_ARTICLES},
	},
	GRAMMAR_INTENT_LANDING_PAGE_ARTICLES: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_ARTICLE, CT_PUBLICATION, CT_ARTICLES},
	},
	GRAMMAR_INTENT_LANDING_PAGE_DOWNLOADS: nil,
	GRAMMAR_INTENT_LANDING_PAGE_HELP:      nil,

	GRAMMAR_INTENT_SOURCE_POSITION_WITHOUT_TERM: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_LESSON_PART, CT_FULL_LESSON, CT_VIDEO_PROGRAM_CHAPTER, CT_SOURCE, CT_DAILY_LESSON, CT_VIDEO_PROGRAM},
	},

	GRAMMAR_INTENT_PROGRAM_POSITION_WITHOUT_TERM: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_VIDEO_PROGRAM_CHAPTER, CT_VIDEO_PROGRAM},
	},

	// Filters

	GRAMMAR_INTENT_FILTER_BY_CONTENT_TYPE: nil,

	GRAMMAR_INTENT_FILTER_BY_SOURCE: map[string][]string{
		FILTER_CONTENT_TYPE: []string{
			CT_CONGRESS,
			CT_DAILY_LESSON,
			CT_EVENT_PART,
			CT_FRIENDS_GATHERING,
			CT_FRIENDS_GATHERINGS,
			CT_FULL_LESSON,
			CT_HOLIDAY,
			CT_LECTURE,
			CT_LECTURE_SERIES,
			CT_LECTURE_SERIES,
			CT_LESSON_PART,
			CT_MEAL,
			CT_MEALS,
			CT_SOURCE,
			CT_UNITY_DAY,
			CT_VIDEO_PROGRAM,
			CT_VIDEO_PROGRAM_CHAPTER,
			CT_VIRTUAL_LESSON,
			CT_VIRTUAL_LESSONS,
			CT_WOMEN_LESSON,
		},
	},

	GRAMMAR_INTENT_FILTER_BY_PROGRAM: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_VIDEO_PROGRAM_CHAPTER, CT_VIDEO_PROGRAM},
	},

	GRAMMAR_INTENT_FILTER_BY_PROGRAM_WITHOUT_TERM: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_VIDEO_PROGRAM_CHAPTER, CT_VIDEO_PROGRAM},
	},
}

const (

	// Variable types

	VAR_YEAR                = "$Year"
	VAR_CONVENTION_LOCATION = "$ConventionLocation"
	VAR_TEXT                = "$Text"
	VAR_HOLIDAYS            = "$Holidays"
	VAR_CONTENT_TYPE        = "$ContentType"
	VAR_SOURCE              = "$Source"
	VAR_POSITION            = "$Position"
	VAR_DIVISION_TYPE       = "$DivisionType"
	VAR_PROGRAM             = "$Program"
	VAR_RESTRICTED          = "$Restricted" // Search terms that privent triggering grammar engine.

	// $ContentType variable values

	VAR_CT_PROGRAMS        = "programs"
	VAR_CT_ARTICLES        = "articles"
	VAR_CT_LESSONS         = "lessons"
	VAR_CT_CLIPS           = "clips"
	VAR_CT_SOURCES         = "sources"
	VAR_CT_BOOK_TITLES     = "books_titles"
	VAR_CT_MEALS           = "meals"
	VAR_CT_BLOG            = "blog"
	VAR_CT_VIRTUAL_LESSONS = "virtual_lessons"
	VAR_CT_WOMEN_LESSONS   = "women_lessons"

	// TBD if the imp. is needed
	/*
		VAR_CT_TWEETS   	= "tweets"
		VAR_CT_PUBLICATIONS = "publications"
		VAR_CT_EVENTS       = "events"
		VAR_CT_HOLIDAYS     = "holidays"
		VAR_CT_CONVENTIONS  = "conventions"'
	*/

	// $DivisionType variable values

	VAR_DIV_ARTICLE = "article"
	VAR_DIV_CHAPTER = "chapter"
	VAR_DIV_VOLUME  = "volume"
	VAR_DIV_PART    = "part"
	VAR_DIV_NUMBER  = "number"
)

// Grammar $ContentType variables to content type filters mapping.
var CT_VARIABLE_TO_FILTER_VALUES = map[string]map[string][]string{
	VAR_CT_PROGRAMS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_VIDEO_PROGRAM_CHAPTER, CT_VIDEO_PROGRAM},
	},
	VAR_CT_ARTICLES: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_ARTICLE, CT_ARTICLES},
	},
	VAR_CT_LESSONS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_LESSON_PART, CT_FULL_LESSON, CT_DAILY_LESSON},
	},
	VAR_CT_CLIPS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_CLIP},
	},
	VAR_CT_MEALS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_MEAL, CT_MEALS},
	},
	VAR_CT_BLOG: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_BLOG_POST},
	},
	VAR_CT_VIRTUAL_LESSONS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_VIRTUAL_LESSON, CT_VIRTUAL_LESSONS},
	},
	VAR_CT_WOMEN_LESSONS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{CT_WOMEN_LESSON, CT_WOMEN_LESSONS},
	},
	/*VAR_CT_TWEETS: map[string][]string{
		FILTER_CONTENT_TYPE: []string{SCT_TWEET},
	},*/
}

var CT_VARIABLES_ENABLE_SOURCES_SEARCH = map[string]bool{
	VAR_CT_ARTICLES:    true, // Article is also source (like 'Maamar Ha-Arvut')
	VAR_CT_SOURCES:     true,
	VAR_CT_BOOK_TITLES: true,
}

// Variable name to frontend filter name mapping.
var VARIABLE_TO_FILTER = map[string]string{
	VAR_YEAR:                "year",
	VAR_CONVENTION_LOCATION: "location",
	VAR_TEXT:                "text",
	VAR_HOLIDAYS:            "holidays",
	VAR_CONTENT_TYPE:        "content_type",
	VAR_SOURCE:              "source",
	VAR_POSITION:            "position",
	VAR_PROGRAM:             "program",
}

// Latency log
const (
	LAT_DOSEARCH                                = "DoSearch"
	LAT_DOSEARCH_MULTISEARCHDO                  = "DoSearch.MultisearchDo"
	LAT_DOSEARCH_MULTISEARCHHIGHLIGHTSDO        = "DoSearch.MultisearcHighlightsDo"
	LAT_DOSEARCH_ADDINTENTS                     = "DoSearch.AddIntents"
	LAT_DOSEARCH_ADDINTENTS_FIRSTROUNDDO        = "DoSearch.AddIntents.FirstRoundDo"
	LAT_DOSEARCH_ADDINTENTS_SECONDROUNDDO       = "DoSearch.AddIntents.SecondRoundDo"
	LAT_DOSEARCH_MULTISEARCHTWEETSDO            = "DoSearch.MultisearchTweetsDo"
	LAT_DOSEARCH_TYPOSUGGESTDO                  = "DoSearch.TypoSuggestDo"
	LAT_GETSUGGESTIONS                          = "GetSuggestions"
	LAT_SUGGEST_SUGGESTIONS                     = "GetSuggestions.SuggestSuggestions"
	LAT_GETSUGGESTIONS_MULTISEARCHDO            = "GetSuggestions.MultisearchDo"
	LAT_DOSEARCH_GRAMMARS_MULTISEARCHGRAMMARSDO = "DoSearch.SearchGrammars.MultisearchGrammarsDo"
	LAT_DOSEARCH_GRAMMARS_MULTISEARCHFILTERDO   = "DoSearch.SearchGrammars.MultisearchFilterDo"
	LAT_DOSEARCH_GRAMMARS_RESULTSTOINTENTS      = "DoSearch.SearchGrammars.ResultsToIntents"
	LAT_GET_SOURCE_COUNTS                       = "GetSourceCounts"
)

var LATENCY_LOG_OPERATIONS_FOR_SEARCH = []string{
	LAT_DOSEARCH,
	LAT_DOSEARCH_MULTISEARCHDO,
	LAT_DOSEARCH_MULTISEARCHHIGHLIGHTSDO,
	LAT_DOSEARCH_ADDINTENTS,
	LAT_DOSEARCH_ADDINTENTS_FIRSTROUNDDO,
	LAT_DOSEARCH_ADDINTENTS_SECONDROUNDDO,
	LAT_DOSEARCH_MULTISEARCHTWEETSDO,
	LAT_DOSEARCH_TYPOSUGGESTDO,
	LAT_DOSEARCH_GRAMMARS_MULTISEARCHGRAMMARSDO,
	LAT_DOSEARCH_GRAMMARS_MULTISEARCHFILTERDO,
}

const (
	PROGRAM_COLLECTION_HAPITARON            = "MvD8Dk2S"
	PROGRAM_COLLECTION_NEW_LIFE             = "zf4lLwyI"
	PROGRAM_COLLECTION_WRITERS_MEETING      = "CwdCR0xR"
	PROGRAM_COLLECTION_EL_MUNDO             = "t9oGIcN5"
	PROGRAM_COLLECTION_NASHIM_BEOLAM_HADASH = "2v7UzleG"
	PROGRAM_COLLECTION_GLOBAL_PERSPECTIVES  = "n0xqPUd0"
	PROGRAM_COLLECTION_NEWS_IN_RUSSIAN      = "SmssHWWs"

	ARTICLE_COLLECTION_NEW_LIFE             = "suS7S4KN"
	ARTICLE_COLLECTION_WRITERS_MEETING      = "dHLMoKlp"
	ARTICLE_COLLECTION_EL_MUNDO             = "QjclHot5"
	ARTICLE_COLLECTION_NASHIM_BEOLAM_HADASH = "Rqd7RNhm"
	ARTICLE_COLLECTION_GLOBAL_PERSPECTIVES  = "1UDYTmy3"
	ARTICLE_COLLECTION_NEWS_IN_RUSSIAN      = "xW7rPwDd"
)

const (
	SRC_SHAMATI                               = "qMUUn22b"
	SRC_NONE_ELSE_BESIDE_HIM                  = "hFeGidcS"
	SRC_PEACE_ARCTICLE                        = "28Cmp7gl"
	SRC_PEACE_IN_WORLD_ARTICLE                = "hqUTKcZz"
	SRC_ARVUT_ARTICLE                         = "itcVAcFn"
	SRC_RABASH_ASSORTED_NOTES                 = "2GAdavz0"
	SRC_THE_ROSE_ARTICLE                      = "yUcfylRm"
	SRC_LETTERS_RABASH                        = "b8SHlrfH"
	SRC_ARTICLES_RABASH                       = "rQ6sIUZK"
	SRC_ARTICLES_BAAL_SULAM                   = "qMeV5M3Y"
	SRC_BAAL_SULAM_ARTICLES_LETTERS_SUMMARIES = "QUBP2DYe"
	SRC_BAAL_SULAM_WRITINGS_CAMPUS_RU         = "8Y0f8Jg9"
	SRC_CONNECTING_TO_THE_SOURCE              = "wWm6fbn4"
	SRC_ZOHAR_FOR_ALL_VOL_12                  = "oau8jvcD"
	SRC_INTRO_TO_BOOK_OF_ZOHAR                = "ALlyoveA"
)

var ES_SUGGEST_SOURCES_WEIGHT = map[string]float64{
	SRC_NONE_ELSE_BESIDE_HIM:   200,
	SRC_PEACE_ARCTICLE:         120,
	SRC_PEACE_IN_WORLD_ARTICLE: 120,
	SRC_ARVUT_ARTICLE:          210,
	SRC_INTRO_TO_BOOK_OF_ZOHAR: 200,
}

// We used to name this articles with the prefix word "maamar" (article).
// We will suggest the correct source result when the user types their name with the prefix "maamar".
var ES_SRC_ADD_MAAMAR_TO_SUGGEST = map[string]bool{
	SRC_PEACE_ARCTICLE:         true,
	SRC_PEACE_IN_WORLD_ARTICLE: true,
	SRC_ARVUT_ARTICLE:          true,
	SRC_THE_ROSE_ARTICLE:       true,
}

type PositionIndexType int

const (
	ALWAYS_NUMBER    PositionIndexType = iota
	LETTER_IF_HEBREW PositionIndexType = iota
)

var ES_SRC_PARENTS_FOR_CHAPTER_POSITION_INDEX = map[string]PositionIndexType{
	SRC_SHAMATI:               LETTER_IF_HEBREW,
	SRC_RABASH_ASSORTED_NOTES: ALWAYS_NUMBER,
}

var ES_GRAMMAR_DIVT_TYPE_TO_SOURCE_TYPES = map[string][]int64{
	VAR_DIV_ARTICLE: []int64{SRC_TYPE_ARTICLE},
	VAR_DIV_CHAPTER: []int64{SRC_TYPE_CHAPTER, SRC_TYPE_ARTICLE, SRC_TYPE_LETTER},
	VAR_DIV_VOLUME:  []int64{SRC_TYPE_VOLUME},
	VAR_DIV_PART:    []int64{SRC_TYPE_PART},
	VAR_DIV_NUMBER:  []int64{SRC_TYPE_CHAPTER, SRC_TYPE_ARTICLE, SRC_TYPE_LETTER},
}

var ES_GRAMMAR_PROGRAM_SUPPORTED_DIV_TYPES = map[string]bool{
	VAR_DIV_CHAPTER: true,
	VAR_DIV_NUMBER:  true,
}

// Note: Suggest is not indexed anyway for rules with "free text" variables.
var ES_SUGGEST_SUPPORTED_GRAMMAR_RULES = map[string]bool{
	GRAMMAR_INTENT_LANDING_PAGE_LESSONS:            true,
	GRAMMAR_INTENT_LANDING_PAGE_VIRTUAL_LESSONS:    true,
	GRAMMAR_INTENT_LANDING_PAGE_LECTURES:           true,
	GRAMMAR_INTENT_LANDING_PAGE_WOMEN_LESSONS:      true,
	GRAMMAR_INTENT_LANDING_PAGE_RABASH_LESSONS:     true,
	GRAMMAR_INTENT_LANDING_PAGE_LESSON_SERIES:      true,
	GRAMMAR_INTENT_LANDING_PAGE_PRORGRAMS:          true,
	GRAMMAR_INTENT_LANDING_PAGE_CLIPS:              true,
	GRAMMAR_INTENT_LANDING_PAGE_LIBRARY:            true,
	GRAMMAR_INTENT_LANDING_PAGE_GROUP_ARTICLES:     true,
	GRAMMAR_INTENT_LANDING_PAGE_CONVENTIONS:        true,
	GRAMMAR_INTENT_LANDING_PAGE_HOLIDAYS:           true,
	GRAMMAR_INTENT_LANDING_PAGE_UNITY_DAYS:         true,
	GRAMMAR_INTENT_LANDING_PAGE_FRIENDS_GATHERINGS: true,
	GRAMMAR_INTENT_LANDING_PAGE_MEALS:              true,
	GRAMMAR_INTENT_LANDING_PAGE_TOPICS:             true,
	GRAMMAR_INTENT_LANDING_PAGE_BLOG:               true,
	GRAMMAR_INTENT_LANDING_PAGE_TWITTER:            true,
	GRAMMAR_INTENT_LANDING_PAGE_ARTICLES:           true,
	GRAMMAR_INTENT_LANDING_PAGE_DOWNLOADS:          true,
	GRAMMAR_INTENT_LANDING_PAGE_HELP:               true,
}

var NOT_TO_INCLUDE_IN_SOURCE_BY_POSITION = []string{
	SRC_LETTERS_RABASH, SRC_ARTICLES_RABASH, SRC_ARTICLES_BAAL_SULAM, // Children 'position' value of these sources are not according to their actual chapter
}

// Source names that we avoid adding:
// 1. Zohar For All vol.12 chapter names (that are actually general topic names).
// 2. Source names from Rabash Assorted notes because many of them are similar to concepts or topics and less known as names of Rabash sources.
// 3. Article summaries and campus material to avoid confusion with the original sources.
var SOURCE_PARENTS_NOT_TO_INCLUDE_IN_VARIABLE_VALUES = []string{
	SRC_RABASH_ASSORTED_NOTES, SRC_ZOHAR_FOR_ALL_VOL_12, SRC_BAAL_SULAM_ARTICLES_LETTERS_SUMMARIES, SRC_BAAL_SULAM_WRITINGS_CAMPUS_RU, SRC_CONNECTING_TO_THE_SOURCE,
}

var ARTICLE_COLLECTION_TO_PROGRAM_COLLECTION = map[string]string{
	ARTICLE_COLLECTION_NEW_LIFE:             PROGRAM_COLLECTION_NEW_LIFE,
	ARTICLE_COLLECTION_WRITERS_MEETING:      PROGRAM_COLLECTION_WRITERS_MEETING,
	ARTICLE_COLLECTION_EL_MUNDO:             PROGRAM_COLLECTION_EL_MUNDO,
	ARTICLE_COLLECTION_NASHIM_BEOLAM_HADASH: PROGRAM_COLLECTION_NASHIM_BEOLAM_HADASH,
	ARTICLE_COLLECTION_GLOBAL_PERSPECTIVES:  PROGRAM_COLLECTION_GLOBAL_PERSPECTIVES,
	ARTICLE_COLLECTION_NEWS_IN_RUSSIAN:      PROGRAM_COLLECTION_NEWS_IN_RUSSIAN,
}

var PERMITTED_UNIT_CT_FOR_DERIVED_FILTER = map[string]bool{
	CT_LIKUTIM:     true,
	CT_SOURCE:      true,
	CT_KITEI_MAKOR: true,
	CT_SONG:        true,
}

var CT_UNITS_MEDIA = []string{
	CT_CLIP,
	CT_VIDEO_PROGRAM_CHAPTER,
	CT_EVENT_PART,
	CT_FRIENDS_GATHERING,
	CT_LECTURE,
	CT_LESSON_PART,
	CT_WOMEN_LESSON,
	CT_VIRTUAL_LESSON,
	CT_MEAL,
}

var CT_UNITS_TEXTS = []string{
	CT_ARTICLE,
	CT_BLOG_POST,
	CT_BOOK,
	CT_PUBLICATION,
	CT_RESEARCH_MATERIAL,
	CT_LIKUTIM,
	CT_SOURCE,
}
