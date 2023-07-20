package api

import (
	"time"

	"github.com/volatiletech/null/v8"

	"github.com/Bnei-Baruch/archive-backend/search"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type BaseRequest struct {
	Language string `json:"language" form:"language" binding:"omitempty,len=2"`
}

type ListRequest struct {
	BaseRequest
	PageNumber int    `json:"page_no" form:"page_no" binding:"omitempty,min=1"`
	PageSize   int    `json:"page_size" form:"page_size" binding:"omitempty,min=1"`
	StartIndex int    `json:"start_index" form:"start_index" binding:"omitempty,min=1"`
	StopIndex  int    `json:"stop_index" form:"stop_index" binding:"omitempty,min=1"`
	OrderBy    string `json:"order_by" form:"order_by" binding:"omitempty"`
	GroupBy    string `json:"-"`
}

type ListResponse struct {
	Total int64 `json:"total"`
}

type ItemRequest struct {
	BaseRequest
	UID string
}

type TagDashboardRequest struct {
	ListRequest
	TagsFilter
	DateRangeFilter
	SourcesFilter
	ContentTypesFilter
	MediaLanguageFilter
}

type IDsFilter struct {
	IDs []string `json:"ids" form:"id" binding:"omitempty"`
}

type ContentTypesFilter struct {
	ContentTypes []string `json:"content_types" form:"content_type" binding:"omitempty"`
}

type DerivedTypesFilter struct {
	DerivedTypes []string `json:"derived_types" form:"derived_types" binding:"omitempty"`
}

type SourcesFilter struct {
	Authors []string `json:"authors" form:"author" binding:"omitempty"`
	Sources []string `json:"sources" form:"source" binding:"omitempty,dive,len=8"`
}

type TagsFilter struct {
	Tags      []string `json:"tags" form:"tag" binding:"omitempty,dive,len=8"`
	RootsOnly bool     `json:"roots_only" form:"roots_only" binding:"omitempty"`
}

type DateRangeFilter struct {
	StartDate string `json:"start_date" form:"start_date" binding:"omitempty"`
	EndDate   string `json:"end_date" form:"end_date" binding:"omitempty"`
}

type ContentUnitsFilter struct {
	ContentUnitIDs []string `json:"content_units" form:"content_unit" binding:"omitempty"`
}

func (drf *DateRangeFilter) Range() (time.Time, time.Time, error) {
	var err error
	var s, e time.Time

	if drf.StartDate != "" {
		s, err = time.Parse("2006-01-02", drf.StartDate)
	}
	if err == nil && drf.EndDate != "" {
		e, err = time.Parse("2006-01-02", drf.EndDate)
		if err == nil {
			e = e.Add(24*time.Hour - 1) // make the hour 23:59:59.999999999
		}
	}

	return s, e, err
}

type GenresProgramsFilter struct {
	Genres   []string `json:"genres" form:"genre" binding:"omitempty"`
	Programs []string `json:"programs" form:"program" binding:"omitempty,dive,len=8"`
}

type CollectionsFilter struct {
	Collections []string `json:"collections" form:"collection" binding:"omitempty,dive,len=8"`
}

type PublishersFilter struct {
	Publishers []string `json:"publishers" form:"publisher" binding:"omitempty,dive,len=8"`
}

type PersonsFilter struct {
	Persons []string `json:"persons" form:"person" binding:"omitempty,dive,len=8"`
}

type UsernameFilter struct {
	Usernames []string `json:"usernames" form:"username" binding:"omitempty"`
}

type BlogFilter struct {
	Blogs []string `json:"blogs" form:"blog" binding:"omitempty"`
}

type MediaLanguageFilter struct {
	MediaLanguage []string `json:"media_languages" form:"media_language" binding:"omitempty,dive,len=2"`
}

type OriginalLanguageFilter struct {
	OriginalLanguages []string `json:"original_languages" form:"original_language" binding:"omitempty,dive,len=2"`
}

type LocationsFilter struct {
	Locations []string `json:"locations" form:"location" binding:"omitempty"`
}

// MediaTypeFilter TODO: for version 1.8 can try to use oneof=text image video validation
type MediaTypeFilter struct {
	MediaType []string `json:"media_types" form:"media_type" binding:"omitempty"`
}

type CuNameFilter struct {
	CuName string `json:"q" form:"q" binding:"omitempty"`
}

type CollectionsRequest struct {
	ListRequest
	IDsFilter
	ContentTypesFilter
	DateRangeFilter
	SourcesFilter
	TagsFilter
	WithUnits bool `json:"with_units" form:"with_units"`
}

type CollectionsResponse struct {
	ListResponse
	Collections []*Collection `json:"collections" binding:"omitempty,dive,len=2"`
}

type MobileContentUnitResponseItem struct {
	ContentUnitUid string     `json:"contentUnitId"`
	CollectionId   *string    `json:"collectionId"`
	Image          string     `json:"image"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	ContentType    string     `json:"contentType"`
	Views          *int64     `json:"views,omitempty"`
	Date           *time.Time `json:"date,omitempty"`
	StartDate      *time.Time `json:"startDate"`
	EndDate        *time.Time `json:"endDate"`
	Duration       *int64     `json:"duration"`
	Number         int        `json:"number,omitempty"`

	internalUnitId       int64   `json:"-"`
	internalCollectionId *int64  `json:"-"`
	tag                  *string `json:"-"`
}

type LessonOverviewRequest struct {
	ListRequest
	ContentTypesFilter
	IDsFilter
	DateRangeFilter
	SourcesFilter
	TagsFilter
	MediaLanguageFilter
	DerivedTypesFilter
	CollectionsFilter
	PersonsFilter
	MediaTypeFilter
	OriginalLanguageFilter
}

type MobileContentUnitResponse struct {
	ListResponse
	Items []*MobileContentUnitResponseItem `json:"items"`
}

type MobileSearchResponse struct {
	Total int64                       `json:"total"`
	Items []*MobileSearchResponseItem `json:"items"`
}

type MobileSearchResponseItem struct {
	ContentUnitUid *string    `json:"contentUnitId,omitempty"`
	CollectionUid  *string    `json:"collectionId,omitempty"`
	SourceUid      *string    `json:"sourceId,omitempty"`
	TagUid         *string    `json:"tagId,omitempty"`
	Title          string     `json:"title"`
	Image          *string    `json:"image,omitempty"`
	Views          *int64     `json:"views,omitempty"`
	Type           string     `json:"contentType"`
	Date           *time.Time `json:"date,omitempty"`
}

type MobileProgramsPageRequest struct {
	ListRequest
	ContentTypesFilter
	SourcesFilter
	TagsFilter
	PersonsFilter
	// more filters should be added here later
}

func NewEmptyLessonOverviewResponse() *MobileContentUnitResponse {
	return &MobileContentUnitResponse{
		ListResponse: ListResponse{Total: 0},
		Items:        make([]*MobileContentUnitResponseItem, 0),
	}
}

type ContentUnitsRequest struct {
	ListRequest
	IDsFilter
	ContentTypesFilter
	DateRangeFilter
	SourcesFilter
	TagsFilter
	GenresProgramsFilter
	CollectionsFilter
	PublishersFilter
	PersonsFilter
	MediaLanguageFilter
	DerivedTypesFilter
	OriginalLanguageFilter
	MediaTypeFilter
	CuNameFilter
	WithFiles       bool `json:"with_files" form:"with_files"`
	WithDerivations bool `json:"with_derivations" form:"with_derivations"`
	WithTags        bool `json:"with_tags" form:"with_tags"`
	WithSources     bool `json:"with_sources" form:"with_sources"`
}

type ContentUnitsResponse struct {
	ListResponse
	ContentUnits []*ContentUnit `json:"content_units"`
}

type LessonsRequest struct {
	ListRequest
	DateRangeFilter
	SourcesFilter
	TagsFilter
	MediaLanguageFilter
	DerivedTypesFilter
	ContentTypesFilter
	CollectionsFilter
	PersonsFilter
	MediaTypeFilter
	OriginalLanguageFilter
}

type EventsRequest struct {
	ListRequest
	DateRangeFilter
	SourcesFilter
	TagsFilter
	MediaLanguageFilter
	ContentTypesFilter
	OriginalLanguageFilter
	LocationsFilter
	CollectionsFilter
}

type LessonsResponseItem struct {
	UID         string `json:"id"`
	ContentType string `json:"content_type"`
}
type LessonsResponse struct {
	ListResponse
	Items []*LessonsResponseItem `json:"items"`
}

type PublishersRequest struct {
	ListRequest
}

type PublishersResponse struct {
	ListResponse
	Publishers []*Publisher `json:"publishers"`
}

type HierarchyRequest struct {
	BaseRequest
	RootUID string `json:"root" form:"root" binding:"omitempty,len=8"`
	Depth   int    `json:"depth" form:"depth"`
}

type HomeResponse struct {
	LatestDailyLesson  *Collection    `json:"latest_daily_lesson"`
	LatestContentUnits []*ContentUnit `json:"latest_units"`
	LatestCollections  []*Collection  `json:"latest_cos"`
	Banner             *Banner        `json:"banner"`
}

type TagsDashboardItem struct {
	LabelID       string `json:"label_id,omitempty"`
	ContentUnitID string `json:"content_unit_id,omitempty"`
	CollectionId  string `json:"collection_id,omitempty"`
	IsText        bool   `json:"is_text,required"`
}
type TagsDashboardResponse struct {
	MediaTotal int64                `json:"media_total"`
	TextTotal  int64                `json:"text_total"`
	Items      []*TagsDashboardItem `json:"items"`
}

type StatsFetchOptions struct {
	WithSources           bool `json:"with_sources" form:"with_sources" binding:"omitempty"`
	WithTags              bool `json:"with_tags" form:"with_tags" binding:"omitempty"`
	WithCollections       bool `json:"with_collections" form:"with_collections" binding:"omitempty"`
	WithLanguages         bool `json:"with_languages" form:"with_languages" binding:"omitempty"`
	WithContentTypes      bool `json:"with_content_types" form:"with_content_types" binding:"omitempty"`
	WithPersons           bool `json:"with_persons" form:"with_persons" binding:"omitempty"`
	WithMediaType         bool `json:"with_media" form:"with_media" binding:"omitempty"`
	WithOriginalLanguages bool `json:"with_original_languages" form:"with_original_languages" binding:"omitempty"`
	WithLocations         bool `json:"with_locations" form:"with_locations" binding:"omitempty"`
}

type StatsClassRequest struct {
	ListRequest
	IDsFilter
	ContentTypesFilter
	DateRangeFilter
	SourcesFilter
	TagsFilter
	CollectionsFilter
	MediaLanguageFilter
	GenresProgramsFilter
	PublishersFilter
	PersonsFilter
	StatsFetchOptions
	MediaTypeFilter
	OriginalLanguageFilter
	LocationsFilter
	CuNameFilter
	CountOnly bool `json:"count_only" form:"count_only"`
	ForFilter bool `json:"for_filter" form:"for_filter"`
}

type StatsClassResponse struct {
	Sources           map[string]int      `json:"sources"`
	Tags              map[string]int      `json:"tags"`
	Languages         map[string]int      `json:"languages"`
	ContentTypes      map[string]int      `json:"content_types"`
	Collections       map[string]int      `json:"collections"`
	Persons           map[string]int      `json:"persons"`
	MediaTypes        map[string]int      `json:"media_types"`
	OriginalLanguages map[string]int      `json:"original_languages"`
	Locations         map[string]CityItem `json:"locations"`
	Total             int64               `json:"total"`
}

type TweetsRequest struct {
	ListRequest
	DateRangeFilter
	UsernameFilter
	IDsFilter
}

type TweetsResponse struct {
	ListResponse
	Tweets []*Tweet `json:"tweets"`
}

type BlogPostsRequest struct {
	ListRequest
	DateRangeFilter
	BlogFilter
	IDsFilter
}

type BlogPostsResponse struct {
	ListResponse
	Posts []*BlogPost `json:"posts"`
}

type SimpleModeRequest struct {
	BaseRequest
	DateRangeFilter
}

type SimpleModeResponse struct {
	ListResponse
	Lessons []*Collection  `json:"lessons"`
	Others  []*ContentUnit `json:"others"`
}

type EvalQueryRequest struct {
	serverUrl          string           `json:"server_url"`
	EvalQuery          search.EvalQuery `json:"eval_query"`
	ExpectationStrings []string         `json:"expectation_strings"`
}

type EvalQueryResponse struct {
	EvalResult search.EvalResult `json:"eval_result"`
}

type EvalSetRequest struct {
	ServerUrl    string `json:"server_url"`
	RecallSetCSV string `json:"recall_set_csv"`
}

type EvalSetResponse struct {
	Results    search.EvalResults    `json:"results"`
	Losses     map[int][]search.Loss `json:"losses"`
	FlatReport string                `json:"flat_report"`
}

type EvalSxSRequest struct {
	DiffsLimit    int32  `json:"diffs_limit"`
	BaseServerUrl string `json:"base_server_url"`
	ExpServerUrl  string `json:"exp_server_url"`
	Language      string `json:"language"`
}

type LabelsRequest struct {
	ListRequest
	IDsFilter
	TagsFilter
	ContentUnitsFilter
	WithTags bool `json:"with_tags" form:"with_tags"`
}

type LabelsResponse struct {
	ListResponse
	Labels []*Label `json:"labels"`
}

func NewCollectionsResponse() *CollectionsResponse {
	return &CollectionsResponse{Collections: make([]*Collection, 0)}
}

func NewContentUnitsResponse() *ContentUnitsResponse {
	return &ContentUnitsResponse{ContentUnits: make([]*ContentUnit, 0)}
}

func NewPublishersResponse() *PublishersResponse {
	return &PublishersResponse{Publishers: make([]*Publisher, 0)}
}

func NewTweetsResponse() *TweetsResponse {
	return &TweetsResponse{Tweets: make([]*Tweet, 0)}
}

func NewBlogPostsResponse() *BlogPostsResponse {
	return &BlogPostsResponse{Posts: make([]*BlogPost, 0)}
}

func NewTagsDashboardResponse() *TagsDashboardResponse {
	return &TagsDashboardResponse{Items: make([]*TagsDashboardItem, 0)}
}

func NewStatsClassResponse() *StatsClassResponse {
	return &StatsClassResponse{
		Sources:           make(map[string]int),
		Tags:              make(map[string]int),
		Languages:         make(map[string]int),
		ContentTypes:      make(map[string]int),
		Collections:       make(map[string]int),
		Persons:           make(map[string]int),
		MediaTypes:        make(map[string]int),
		OriginalLanguages: make(map[string]int),
		Total:             0,
	}
}

func NewLabelsResponse() *LabelsResponse {
	return &LabelsResponse{Labels: make([]*Label, 0)}
}

type Collection struct {
	ID              string         `json:"id"`
	ContentType     string         `json:"content_type"`
	Name            string         `json:"name,omitempty"`
	Description     string         `json:"description,omitempty"`
	FilmDate        *utils.Date    `json:"film_date,omitempty"`
	StartDate       *utils.Date    `json:"start_date,omitempty"`
	EndDate         *utils.Date    `json:"end_date,omitempty"`
	Country         string         `json:"country,omitempty"`
	City            string         `json:"city,omitempty"`
	FullAddress     string         `json:"full_address,omitempty"`
	Genres          []string       `json:"genres,omitempty"`
	DefaultLanguage string         `json:"default_language,omitempty"`
	HolidayID       string         `json:"holiday_id,omitempty"`
	SourceID        string         `json:"source_id,omitempty"`
	TagIDs          []string       `json:"tag_id,omitempty"`
	Number          int            `json:"number,omitempty"`
	ContentUnits    []*ContentUnit `json:"content_units,omitempty"`
}

type ContentUnit struct {
	mdbID            int64
	ID               string                 `json:"id"`
	ContentType      string                 `json:"content_type"`
	NameInCollection string                 `json:"name_in_collection,omitempty"`
	FilmDate         *utils.Date            `json:"film_date,omitempty"`
	Name             string                 `json:"name,omitempty"`
	Description      string                 `json:"description,omitempty"`
	Duration         float64                `json:"duration,omitempty"`
	OriginalLanguage string                 `json:"original_language,omitempty"`
	Files            []*File                `json:"files,omitempty"`
	Collections      map[string]*Collection `json:"collections,omitempty"`
	Sources          []string               `json:"sources,omitempty"`
	Tags             []string               `json:"tags,omitempty"`
	tagIDs           []int64
	Publishers       []string                `json:"publishers,omitempty"`
	SourceUnits      map[string]*ContentUnit `json:"source_units,omitempty"`
	DerivedUnits     map[string]*ContentUnit `json:"derived_units,omitempty"`
}

type File struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Size           int64     `json:"size"`
	Duration       float64   `json:"duration,omitempty"`
	Language       string    `json:"language,omitempty"`
	HlsLanguages   []string  `json:"hls_languages,omitempty"`
	MimeType       string    `json:"mimetype,omitempty"`
	Type           string    `json:"type,omitempty"`
	SubType        string    `json:"subtype,omitempty"`
	VideoSize      string    `json:"video_size,omitempty"`
	InsertType     string    `json:"insert_type,omitempty"`
	IsHLS          bool      `json:"is_hls"`
	VideoQualities []string  `json:"video_qualities" binding:"omitempty"`
	CreatedAt      time.Time `json:"-"`
}

type Source struct {
	UID         string     `json:"id"`
	ParentUID   string     `json:"parent_id"`
	Type        string     `json:"type"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Year        string     `json:"year,omitempty"`
	Number      string     `json:"number,omitempty"`
	Children    []*Source  `json:"children,omitempty"`
	ID          int64      `json:"-"`
	ParentID    null.Int64 `json:"-"`
	Position    null.Int   `json:"-"`
}

type Author struct {
	Code     string      `json:"id"`
	Name     string      `json:"name"`
	FullName null.String `json:"full_name,omitempty"`
	Children []*Source   `json:"children,omitempty"`
}

type Tag struct {
	UID       string      `json:"id"`
	ParentUID string      `json:"parent_id"`
	Label     null.String `json:"label"`
	Children  []*Tag      `json:"children,omitempty"`
	ID        int64       `json:"-"`
	ParentID  null.Int64  `json:"-"`
}

type Publisher struct {
	UID         string      `json:"id"`
	Name        null.String `json:"name"`
	Description null.String `json:"description,omitempty"`
	ID          int64       `json:"-"`
}

type Person struct {
	UID  string      `json:"id"`
	Name null.String `json:"name"`
	ID   int64       `json:"-"`
}

type CollectionUpdateStatus struct {
	UID        string     `json:"id"`
	LastUpdate utils.Date `json:"last_update"`
	UnitsCount int        `json:"units_count"`
}

type Banner struct {
	Section   string `json:"section"`
	Header    string `json:"header"`
	SubHeader string `json:"sub_header"`
	Url       string `json:"url"`
	Image     string `json:"image"`
}

type SemiQuasiData struct {
	Authors    []*Author    `json:"sources"`
	Tags       []*Tag       `json:"tags"`
	Publishers []*Publisher `json:"publishers"`
	Persons    []*Person    `json:"persons"`
}

type Tweet struct {
	Username  string    `json:"username"`
	TwitterID string    `json:"twitter_id"`
	FullText  string    `json:"full_text"`
	CreatedAt time.Time `json:"created_at"`
	Raw       null.JSON `json:"raw"`
}

type BlogPost struct {
	Blog         string    `json:"blog"`
	WordpressID  int64     `json:"wp_id"`
	CanonicalUrl string    `json:"url"`
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"created_at"`
}

type Label struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	MediaType   string    `json:"media_type"`
	Properties  null.JSON `json:"properties,omitempty"`
	Author      string    `json:"author,required"`
	TagUIDs     []string  `json:"tags,omitempty"`
	ContentUnit string    `json:"content_unit,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type CityItem struct {
	Count   int    `json:"count"`
	City    string `json:"city"`
	Country string `json:"country"`
}
