package api

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/feeds"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var copyright = fmt.Sprintf("Bnei-Baruch Copyright 2008-%d", time.Now().Year())

// TODO: Feed for Som
func FeedRusZohar(c *gin.Context) {
	feed := &feeds.Feed{
		Title:       "Kabbalah Media Zohar Lesson",
		Link:        getHref("/feeds/rus_zohar.rss", c),
		Description: "The evening Zohar lesson from Kabbalahmedia Archive",
		Updated:     time.Now(),
		Copyright:   copyright,
	}

	db := c.MustGet("MDB_DB").(*sql.DB)

	cur := ContentUnitsRequest{
		ListRequest: ListRequest{
			BaseRequest: BaseRequest{
				Language: consts.LANG_RUSSIAN,
			},
			PageNumber: 1,
			PageSize:   1,
			OrderBy:    "(properties->>'film_date')::date desc, created_at desc",
		},
		ContentTypesFilter: ContentTypesFilter{
			ContentTypes: []string{consts.CT_LESSON_PART},
		},
		SourcesFilter: SourcesFilter{Sources: []string{"AwGBQX2L"}}, // Zohar
	}

	mediaTypes := []string{consts.MEDIA_MP3, consts.MEDIA_MP4}
	languages := []string{consts.LANG_RUSSIAN, consts.LANG_HEBREW}
	item, err := handleContentUnitsFull(db, cur, mediaTypes, languages)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	cu := item.ContentUnits[0]
	videoRus := buildHtmlFromFile(consts.LANG_RUSSIAN, consts.MEDIA_MP4, cu.Files, cu.Duration)
	audioRus := buildHtmlFromFile(consts.LANG_RUSSIAN, consts.MEDIA_MP3, cu.Files, cu.Duration)
	videoHeb := buildHtmlFromFile(consts.LANG_HEBREW, consts.MEDIA_MP4, cu.Files, cu.Duration)
	audioHeb := buildHtmlFromFile(consts.LANG_HEBREW, consts.MEDIA_MP3, cu.Files, cu.Duration)
	link := "https://kabbalahmedia.info/ru/lessons/cu/" + cu.ID

	feed.Items = []*feeds.Item{
		{
			Title: "Урок по Книге Зоар, " + cu.FilmDate.Format("02.01.2006"),
			Guid:  link,
			Link:  link,
			Description: &feeds.Description{Text: fmt.Sprintf(
				`
					<div class="title">
						<h2>%s</h2>
						Видео (рус.): %s Аудио (рус.): %s Видео (ивр.): %s Аудио (ивр.): %s
					</div>
				`, cu.Name, videoRus, audioRus, videoHeb, audioHeb)},
			Created: cu.FilmDate.Time,
		},
	}

	createFeed(feed, "RUS", c)
}

func createFeed(feed *feeds.Feed, language string, c *gin.Context) {
	feed.Language = language
	channel := feed.RssFeed()
	content, err := channel.ToXML()
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	c.Header("Content-Type", "application/rss+xml; charset=utf-8")
	c.String(http.StatusOK, content)
}

// TODO: Feed for Som
func FeedRusForLaitmanRu(c *gin.Context) {
	feed := &feeds.Feed{
		Title:       "Kabbalah Media Morning Lesson",
		Link:        getHref("/feeds/rus_for_laitman_ru.rss", c),
		Description: "The last lesson from Kabbalamedia Archive",
		Updated:     time.Now(),
		Copyright:   copyright,
	}

	db := c.MustGet("MDB_DB").(*sql.DB)
	items, herr := handleLatestLesson(db, BaseRequest{Language: consts.LANG_RUSSIAN}, true, true)
	if herr != nil {
		herr.Abort(c)
	}

	feed.Items = make([]*feeds.Item, len(items.ContentUnits))
	for idx := range items.ContentUnits {
		cu := items.ContentUnits[idx]
		videoRus := buildHtmlFromFile(consts.LANG_RUSSIAN, consts.MEDIA_MP4, cu.Files, cu.Duration)
		audioRus := buildHtmlFromFile(consts.LANG_RUSSIAN, consts.MEDIA_MP3, cu.Files, cu.Duration)
		videoHeb := buildHtmlFromFile(consts.LANG_HEBREW, consts.MEDIA_MP4, cu.Files, cu.Duration)
		audioHeb := buildHtmlFromFile(consts.LANG_HEBREW, consts.MEDIA_MP3, cu.Files, cu.Duration)
		link := "https://kabbalahmedia.info/ru/lessons/cu/" + cu.ID
		feed.Items[idx] = &feeds.Item{
			Title: "Утренний урок " + cu.FilmDate.Format("02.01.2006"),
			Guid:  link,
			Link:  link,
			Description: &feeds.Description{Text: fmt.Sprintf(
				`
					<div class="title">
						<h2>%s</h2>
						Видео (рус.): %s Аудио (рус.): %s Видео (ивр.): %s Аудио (ивр.): %s
					</div>
				`, cu.Name, videoRus, audioRus, videoHeb, audioHeb)},
			Created: cu.FilmDate.Time,
		}
	}

	createFeed(feed, "RUS", c)
}

type feedConfig struct {
	DLANG string
	Lang  string
	DAYS  int64
	CID   int64
	DF    string
	DT    string
}

// wsxml.xml?CID=4016&DLANG=HEB&DF=2013-04-30&DT=2013-03-31
// supports only CID: 120 - yehsivat haverim (kab.co.il), 4728 - lesson parts (kab.co.il)
// This feed is used by kab.co.il
// On that server there is a hardcoded ip of our server !!!
func FeedWSXML(c *gin.Context) {
	var config feedConfig
	(&config).getConfig(c)
	catalogId := config.CID
	dateFrom := config.DF
	dateTo := config.DT

	if catalogId == 0 {
		c.String(http.StatusOK, "<lessons />")
		return
	}
	if len(dateTo) == 0 {
		t := time.Now()
		dateTo = t.Format("2006-01-02")
	}
	if len(dateFrom) == 0 {
		t := time.Now().AddDate(0, -1, 0)
		dateFrom = t.Format("2006-01-02")
	}
	if dateTo < dateFrom {
		dateTo, dateFrom = dateFrom, dateTo
	}

	cur := ContentUnitsRequest{
		ListRequest: ListRequest{
			BaseRequest: BaseRequest{
				Language: config.Lang,
			},
			StartIndex: 1,
			StopIndex:  20,
			OrderBy:    "properties->'filmdate' desc, created_at desc",
		},
		DateRangeFilter: DateRangeFilter{
			StartDate: dateFrom,
			EndDate:   dateTo,
		},
	}
	switch catalogId {
	case 120: // yeshivat-haverim => FRIENDS_GATHERING
		cur.ContentTypesFilter = ContentTypesFilter{
			ContentTypes: []string{consts.CT_FRIENDS_GATHERING},
		}
		break
	case 4728: // lessons-part => LESSON_PART
		cur.ContentTypesFilter = ContentTypesFilter{
			ContentTypes: []string{consts.CT_LESSON_PART},
		}
		break
	default:
		c.String(http.StatusOK, "<lessons />")
		return
	}
	db := c.MustGet("MDB_DB").(*sql.DB)
	item, err := handleContentUnitsFull(db, cur, []string{}, []string{})
	if err != nil {
		if err == sql.ErrNoRows {
			c.String(http.StatusOK, "<lessons />")
		} else {
			NewInternalError(err).Abort(c)
		}
		return
	}
	if len(item.ContentUnits) == 0 {
		c.String(http.StatusOK, "<lessons />")
		return
	}

	type fileT struct {
		XMLName xml.Name `xml:"file"`

		Type     string `xml:"type"`
		Language string `xml:"language"`
		Original int    `xml:"original"`
		Path     string `xml:"path"`
		Size     string `xml:"size"`
		Length   string `xml:"length"`
		Title    string `xml:"title"`
	}

	type filesT struct {
		XMLName xml.Name `xml:"files"`

		Files []fileT
	}

	type lessonT struct {
		XMLName xml.Name `xml:"lesson"`

		Title       string `xml:"title"`
		Description string `xml:"description,omitempty"`
		Link        string `xml:"link"`
		Date        string `xml:"date"`
		Language    string `xml:"language"`
		Lecturer    string `xml:"lecturer"`
		Files       filesT
	}

	type lessonsT struct {
		XMLName xml.Name `xml:"lessons"`

		Lesson []lessonT
	}

	lessons := lessonsT{
		Lesson: make([]lessonT, len(item.ContentUnits)),
	}
	for idx, cu := range item.ContentUnits {
		files := cu.Files
		lessons.Lesson[idx] = lessonT{
			Title:       cu.Name,
			Description: cu.Description,
			Link:        getHref("/"+config.Lang+"/lessons/cu/"+string(cu.ID), c),
			Date:        cu.FilmDate.Format("Mon, 2 Jan 2006 15:04:05 -0700"),
			Language:    consts.LANG2CODE[cu.OriginalLanguage],
			Lecturer:    "",
			Files: filesT{
				Files: make([]fileT, len(files)),
			},
		}
		for j, f := range files {
			language := consts.LANG2CODE[f.Language]
			original := 1
			if f.Language != cu.OriginalLanguage {
				original = 0
			}
			size := fmt.Sprintf("%.2f MB", convertSizeToMb(int64(f.Size)))
			lessons.Lesson[idx].Files.Files[j] = fileT{
				Type:     f.Type,
				Language: language,
				Original: original,
				Path:     fmt.Sprintf("%s%s", consts.CDN, f.ID),
				Size:     size,
				Length:   convertDuration(cu.Duration),
				Title:    language + " " + size,
			}
		}
	}

	c.XML(http.StatusOK, lessons)
}

// Lesson Downloader
// DLANG=ENG&DF=[A]/V&DAYS=[0]|1
func FeedMorningLesson(c *gin.Context) {
	var config feedConfig
	(&config).getConfig(c)
	var date string
	if config.DAYS == 1 {
		date = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	} else {
		date = time.Now().Format("2006-01-02")
	}
	cur := ContentUnitsRequest{
		ListRequest: ListRequest{
			BaseRequest: BaseRequest{
				Language: config.Lang,
			},
			StartIndex: 1,
			StopIndex:  20,
			OrderBy:    "created_at asc",
		},
		ContentTypesFilter: ContentTypesFilter{
			ContentTypes: []string{consts.CT_LESSON_PART},
		},
		DateRangeFilter: DateRangeFilter{
			StartDate: date,
		},
	}

	var mediaTypes []string
	if config.DF == "V" {
		mediaTypes = []string{consts.MEDIA_MP4}
	} else {
		mediaTypes = []string{consts.MEDIA_MP3a, consts.MEDIA_MP3b}
	}

	db := c.MustGet("MDB_DB").(*sql.DB)
	item, err := handleContentUnitsFull(db, cur, mediaTypes, []string{})
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, []string{})
		} else {
			NewInternalError(err).Abort(c)
		}
		return
	}

	type FileFeed struct {
		Name      string
		Url       string
		Size      int64
		VideoSize string
		Created   time.Time
	}
	type Item struct {
		Title    string
		Files    []FileFeed
		Duration string
	}
	var items []Item
	var nameToIgnore = regexp.MustCompile("kitei-makor|lelo-mikud")

	for _, cu := range item.ContentUnits {
		if nameToIgnore.MatchString(cu.Name) {
			continue
		}
		item := Item{
			Title:    cu.Name,
			Duration: convertDuration(cu.Duration),
		}
		var files []FileFeed
		for _, file := range cu.Files {
			if file.Language != config.Lang {
				continue
			}
			files = append(files,
				FileFeed{
					Name:      file.Name,
					Url:       fmt.Sprintf("%s%s", consts.CDN, file.ID),
					Size:      file.Size,
					VideoSize: file.VideoSize,
					Created:   file.CreatedAt,
				})
		}
		item.Files = files
		items = append(items, item)
	}
	c.JSON(http.StatusOK, items)
}

func rssPhpDescription(lang string) string {
	switch lang {
	case "tr":
		return "Burada en son Kabala makale, video, haber, konuları ve Bney Baruh internet sitesi güncelleştirmeleri ve içerikleri bulacaksınız."
	case "he":
		return "כאן תקבלו עדכונים יומיים של שיחות, הרצאות ושיעורי קבלה. התכנים מבוססים על מקורות הקבלה האותנטיים בלבד"
	case "ru":
		return "МЕЖДУНАРОДНАЯ АКАДЕМИЯ КАББАЛЫ - крупнейший в мире учебно-образовательный интернет-ресурс, бесплатный и неограниченный источник получения достоверной информации о науке каббала!"
	case "ua":
		return "МІЖНАРОДНА АКАДЕМІЯ Каббали - найбільший в світі навчально освітній інтернет-ресурс, безкоштовне і необмежене джерело отримання достовірної інформації про науку каббала!"
	default:
		return "Here you will find all the latest Kabbalah articles, videos, audio, news, features, Bnei Baruch website updates and content additions."
	}
}

func FeedRssPhp(c *gin.Context) {
	var config feedConfig
	(&config).getConfig(c)
	feed := &feeds.Feed{
		Title:       "Bnei-Baruch Kabbalahmedia MP3 Podcast",
		Link:        getHref("/rss.php?DLANG="+config.DLANG, c),
		Description: rssPhpDescription(config.Lang),
		Language:    config.Lang,
		Updated:     time.Now(),
		Copyright:   copyright,
	}

	db := c.MustGet("MDB_DB").(*sql.DB)

	cur := ContentUnitsRequest{
		ListRequest: ListRequest{
			BaseRequest: BaseRequest{
				Language: config.Lang,
			},
			StartIndex: 1,
			StopIndex:  20,
			OrderBy:    "created_at desc",
		},
		ContentTypesFilter: ContentTypesFilter{
			ContentTypes: []string{consts.CT_LESSON_PART},
		},
		DateRangeFilter: DateRangeFilter{
			StartDate: time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
		},
	}

	mediaTypes := []string{consts.MEDIA_MP3a, consts.MEDIA_MP3b}
	languages := []string{config.Lang}
	item, err := handleContentUnitsFull(db, cur, mediaTypes, languages)
	if err != nil {
		if err == sql.ErrNoRows {
			createFeed(feed, config.Lang, c)
		} else {
			NewInternalError(err).Abort(c)
		}
		return
	}

	feed.Items = make([]*feeds.Item, 0)
	for _, cu := range item.ContentUnits {
		files := cu.Files
		for _, file := range files {
			url := fmt.Sprintf("%s%s", consts.CDN, file.ID)
			feed.Items = append(feed.Items, &feeds.Item{
				Title: cu.Name,
				Description: &feeds.Description{
					Text: "<div>" + file.Name + "; " + file.CreatedAt.Format("Mon, Jan _2 15:04:05 2006") + " </div>",
				},
				Guid: url,
				Link: url,
				Enclosure: &feeds.Enclosure{
					Url:    url,
					Length: file.Size,
					Type:   file.MimeType,
				},
				Created: file.CreatedAt,
			})
		}
	}

	createFeed(feed, config.Lang, c)
}

func FeedRssVideo(c *gin.Context) {
	var config feedConfig
	(&config).getConfig(c)
	feed := &feeds.Feed{
		Title:       "Kabbalah Media Updates",
		Link:        getHref("/feeds/rss_video.rss?&DLANG="+config.DLANG, c),
		Description: "Video updates from Kabbalamedia Archive",
		Language:    config.Lang,
		Updated:     time.Now(),
		Copyright:   copyright,
	}

	db := c.MustGet("MDB_DB").(*sql.DB)

	cur := ContentUnitsRequest{
		ListRequest: ListRequest{
			BaseRequest: BaseRequest{
				Language: config.Lang,
			},
			StartIndex: 1,
			StopIndex:  20,
			OrderBy:    "created_at asc",
		},
		DateRangeFilter: DateRangeFilter{
			StartDate: time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
		},
	}

	mediaTypes := []string{consts.MEDIA_MP3a, consts.MEDIA_MP3b, consts.MEDIA_MP4}
	item, err := handleContentUnitsFull(db, cur, mediaTypes, []string{})
	if err != nil {
		if err == sql.ErrNoRows {
			createFeed(feed, config.Lang, c)
		} else {
			NewInternalError(err).Abort(c)
		}
		return
	}

	feed.Items = make([]*feeds.Item, 0)
	for _, cu := range item.ContentUnits {
		files := cu.Files

		link := "https://kabbalahmedia.info/ru/lessons/cu/" + cu.ID // TODO
		feed.Items = append(feed.Items, &feeds.Item{
			Title: cu.Name + " " + cu.FilmDate.Format("(02-01-2006)"),
			Guid:  link,
			Link:  link,
			Description: &feeds.Description{
				Text: buildRssVideoHtmlFromFiles(files),
			},
			Created: cu.FilmDate.Time,
		})
	}

	createFeed(feed, config.Lang, c)
}

func showAsset(language string, mimeTypes []string, files []*File, duration float64, name string, ext string) string {
	var bestFiles []*File = nil
	for _, file := range files {
		if file.Language == language && inArray(file.MimeType, mimeTypes) {
			bestFiles = append(bestFiles, file)
		}
	}
	if len(bestFiles) == 0 {
		return ""
	}

	var maxSize int64 = 0
	var maxIndex int
	for idx, file := range bestFiles {
		if file.Size > maxSize {
			maxIndex = idx
			maxSize = file.Size
		}
	}
	file := bestFiles[maxIndex]
	size := convertSizeToMb(file.Size)
	var title string
	if duration == 0 {
		title = fmt.Sprintf("%s&nbsp;|&nbsp;%.2fMb", ext, size)
	} else {
		title = fmt.Sprintf("%s&nbsp;|&nbsp;%.2fMb&nbsp;|&nbsp;%s", ext, size, convertDuration(duration))
	}
	return fmt.Sprintf(`<a href="%s%s" title="%s">%s %s</a>`, consts.CDN, file.ID, title, name, ext)
}

func inArray(val string, array []string) (ok bool) {
	for i := range array {
		if ok = array[i] == val; ok {
			return
		}
	}
	return
}

func buildHtmlFromFile(language string, mimeType string, files []*File, duration float64) string {
	for _, file := range files {
		if file.MimeType == mimeType && file.Language == language {
			size := convertSizeToMb(file.Size)
			var title string
			if duration == 0 {
				title = fmt.Sprintf("mp4&nbsp;|&nbsp;%.2fMb", size)
			} else {
				title = fmt.Sprintf("mp4&nbsp;|&nbsp;%.2fMb&nbsp;|&nbsp;%s", size, convertDuration(duration))
			}
			return fmt.Sprintf(
				`<a href="%s%s" title="%s">Открыть</a> | <a href="%s%s" title="%s">Скачать</a>`,
				consts.CDN, file.ID, title, consts.CDN, file.ID, title)
		}
	}

	return "N/A"
}

func buildRssVideoHtmlFromFiles(files []*File) (items string) {
	var h = map[string][]*File{}
	for _, file := range files {
		key := "<b>" + file.Type + ":</b><br/>"
		h[key] = append(h[key], file)
	}

	items = ""
	for key, f := range h {
		items += key
		for _, file := range f {
			fileSize := convertSizeToMb(file.Size)
			name := fmt.Sprintf("%s_%.2fMB", consts.LANG2CODE[file.Language], fileSize)
			href := fmt.Sprintf("%s%s", consts.CDN, file.ID)

			items += "<div><a href='" + href + "'>" + name + "</a></div>"
		}
	}
	return
}

func convertSizeToMb(size int64) float64 {
	return float64(size) / 1024 / 1024
}

func convertDuration(duration float64) string {
	return time.Unix(int64(duration), 0).UTC().Format("15:04:05")
}

func getHref(href string, c *gin.Context) string {
	return utils.ResolveScheme(c) + "://" + utils.ResolveHost(c) + href
}

func (config *feedConfig) getConfig(c *gin.Context) {
	if c.Bind(config) != nil {
		return
	}
	lang := c.Param("DLANG")
	if config.DLANG == "" && lang != "" {
		config.DLANG = lang
	}
	if config.DLANG == "" {
		config.DLANG = "ENG"
		config.Lang = consts.LANG_ENGLISH
	} else {
		config.Lang = consts.CODE2LANG[config.DLANG]
		if config.Lang == "" {
			config.DLANG = "ENG"
			config.Lang = consts.LANG_ENGLISH
		}
	}
	df := c.Param("DF")
	if config.DF == "" && df != "" {
		config.DF = df
	}
}
