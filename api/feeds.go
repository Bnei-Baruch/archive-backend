package api

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/feeds"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/Bnei-Baruch/sqlboiler/queries/qm"
)

var copyright = fmt.Sprintf("Bnei-Baruch Copyright 2008-%d", time.Now().Year())

func FeedRusZohar(c *gin.Context) {
	var err error

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

	item, herr := handleContentUnits(db, cur)
	if herr != nil {
		herr.Abort(c)
		return
	}
	cu := item.ContentUnits[0]
	id, err := mapCU2ID(cu.ID, db, c)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	fileMap, err := loadCUFiles(db, []int64{id})
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	files, ok := fileMap[id]
	if !ok {
		NewInternalError(errors.Errorf("Illegal state: unit %s not in file map", cu.ID)).Abort(c)
		return
	}

	videoRus := buildHtmlFromFile(consts.LANG_RUSSIAN, consts.MEDIA_MP4, files, cu.Duration)
	audioRus := buildHtmlFromFile(consts.LANG_RUSSIAN, consts.MEDIA_MP3, files, cu.Duration)
	videoHeb := buildHtmlFromFile(consts.LANG_HEBREW, consts.MEDIA_MP4, files, cu.Duration)
	audioHeb := buildHtmlFromFile(consts.LANG_HEBREW, consts.MEDIA_MP3, files, cu.Duration)
	link := "https://archive.kbb1.com/ru/lessons/cu/" + cu.ID

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

	createFeed(feed, "RUS", false, c)
}

func createFeed(feed *feeds.Feed, language string, isItunes bool, c *gin.Context) {
	feed.Language = language
	channel := feed.RssFeed()
	content, err := channel.ToXML(isItunes)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	c.String(http.StatusOK, content)
}

func FeedRusForLaitmanRu(c *gin.Context) {
	var err error

	feed := &feeds.Feed{
		Title:       "Kabbalah Media Morning Lesson",
		Link:        getHref("/feeds/rus_for_laitman_ru.rss", c),
		Description: "The last lesson from Kabbalamedia Archive",
		Updated:     time.Now(),
		Copyright:   copyright,
	}

	db := c.MustGet("MDB_DB").(*sql.DB)
	lessonParts, herr := handleLatestLesson(db, BaseRequest{Language: consts.LANG_RUSSIAN}, true)
	if herr != nil {
		herr.Abort(c)
	}

	cuids := make([]int64, len(lessonParts.ContentUnits))
	for idx, cu := range lessonParts.ContentUnits {
		id, err := mapCU2ID(cu.ID, db, c)
		if err != nil {
			if err == sql.ErrNoRows {
				// empty feed
			} else {
				NewInternalError(err).Abort(c)
			}
			return
		}
		cuids[idx] = id
	}
	fileMap, err := loadCUFiles(db, cuids)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	feed.Items = make([]*feeds.Item, len(lessonParts.ContentUnits))
	for idx, cu := range lessonParts.ContentUnits {
		files, ok := fileMap[cuids[idx]]
		if !ok {
			NewInternalError(errors.Errorf("Illegal state: unit %s not in file map", cu.ID)).Abort(c)
			return
		}
		videoRus := buildHtmlFromFile(consts.LANG_RUSSIAN, consts.MEDIA_MP4, files, cu.Duration)
		audioRus := buildHtmlFromFile(consts.LANG_RUSSIAN, consts.MEDIA_MP3, files, cu.Duration)
		videoHeb := buildHtmlFromFile(consts.LANG_HEBREW, consts.MEDIA_MP4, files, cu.Duration)
		audioHeb := buildHtmlFromFile(consts.LANG_HEBREW, consts.MEDIA_MP3, files, cu.Duration)
		link := "https://archive.kbb1.com/ru/lessons/cu/" + cu.ID
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

	createFeed(feed, "RUS", false, c)
}

func mapCU2ID(cuID string, db *sql.DB, c *gin.Context) (id int64, err error) {
	xu, err := mdbmodels.ContentUnits(db, qm.Where("uid = ?", cuID)).One()
	if err != nil {
		if err == sql.ErrNoRows {
			// empty feed
		} else {
			NewInternalError(err).Abort(c)
		}
		return
	}
	id = xu.ID
	return
}

type feedConfig struct {
	DLANG string
	Lang  string
	DAYS  int64
	CID   int64
	DF    string
	DT    string
}

type translation struct {
	LessonFrom string
	Playlist   string
	Download   string
	Video      string
	Audio      string
}

var T = map[string]translation{
	"ENG": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"HEB": {LessonFrom: "שיעור מתאריך", Playlist: "רשימת השמעה", Download: "הורדת השיעור", Video: "וידאו", Audio: "אודיו"},
	"RUS": {LessonFrom: "Урок от", Playlist: "Плейлист:", Download: "Скачать:", Video: "видео", Audio: "аудио"},
	"SPA": {LessonFrom: "Clase de", Playlist: "Lista de reproducción", Download: "Descargar", Video: "Video", Audio: "Audio"},
	"ITA": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"GER": {LessonFrom: "Unterricht von", Playlist: "Spielliste:", Download: "Downloaden:", Video: "Video", Audio: "Audio"},
	"DUT": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"FRE": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"POR": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"TRK": {LessonFrom: "Dersler", Playlist: "Çalma listesi:", Download: "Yükle:", Video: "Video", Audio: "ses"},
	"POL": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"ARB": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"HUN": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"FIN": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"LIT": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"JPN": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"BUL": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"GEO": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"NOR": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"SWE": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"HRV": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"CHN": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"FAR": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"RON": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"HIN": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"UKR": {LessonFrom: "Урок від", Playlist: "Плейлист:", Download: "Завантажити:", Video: "відео", Audio: "аудіо"},
	"MKD": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"SLV": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"LAV": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"SLK": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
	"CZE": {LessonFrom: "Lesson from", Playlist: "Playlist:", Download: "Download:", Video: "Video", Audio: "Audio"},
}

// wsxml.xml?CID=4016&DLANG=HEB&DF=2013-04-30&DT=2013-03-31
// supports only CID: 120, 3589, 3673, 4016, 4728
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

	cur := ContentUnitsRequest{
		ListRequest: ListRequest{
			BaseRequest: BaseRequest{
				Language: config.Lang,
			},
			StartIndex: 1,
			StopIndex:  150,
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
	case 3673: // tv-clip => CLIP
		cur.ContentTypesFilter = ContentTypesFilter{
			ContentTypes: []string{consts.CT_CLIP},
		}
		break
	case 4016: // lessons_zohar-la-am => LESSON_PART + SourcesFilter.Sources= ["AwGBQX2L"]
		cur.ContentTypesFilter = ContentTypesFilter{
			ContentTypes: []string{consts.CT_LESSON_PART},
		}
		cur.SourcesFilter = SourcesFilter{Sources: []string{"AwGBQX2L"}}
		break
	case 4728: // lessons-part => LESSON_PART
		cur.ContentTypesFilter = ContentTypesFilter{
			ContentTypes: []string{consts.CT_LESSON_PART},
		}
		break
		// vl_heb_virtual-group-israel => ignore
		//case 3589:
		//	break
	default:
		c.String(http.StatusOK, "<lessons />")
		return
	}
	db := c.MustGet("MDB_DB").(*sql.DB)
	items, herr := handleContentUnits(db, cur)
	if herr != nil {
		herr.Abort(c)
	}
	if len(items.ContentUnits) == 0 {
		c.String(http.StatusOK, "<lessons />")
		return
	}

	cuids := make([]int64, len(items.ContentUnits))
	for idx, cu := range items.ContentUnits {
		id, err := mapCU2ID(cu.ID, db, c)
		if err != nil {
			if err == sql.ErrNoRows {
				// empty feed
			} else {
				NewInternalError(err).Abort(c)
			}
			return
		}
		cuids[idx] = id
	}

	fileMap, err := loadCUFiles(db, cuids)
	if err != nil {
		NewInternalError(err).Abort(c)
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
		Lesson: make([]lessonT, len(items.ContentUnits)),
	}
	for i, unit := range items.ContentUnits {
		unitFiles := fileMap[cuids[i]]
		lessons.Lesson[i] = lessonT{
			Title:       unit.Name,
			Description: unit.Description,
			Link:        getHref("/"+config.Lang+"/lessons/cu/"+string(unit.ID), c),
			Date:        unit.FilmDate.String(),
			Language:    consts.LANG2CODE[unit.OriginalLanguage],
			Lecturer:    "",
			Files: filesT{
				Files: make([]fileT, len(unitFiles)),
			},
		}
		for j, f := range unitFiles {
			language := consts.LANG2CODE[f.Language.String]
			original := 1
			if f.Language.String != unit.OriginalLanguage {
				original = 0
			}
			size := fmt.Sprintf("%.2f MB", convertSizeToMb(int64(f.Size)))
			lessons.Lesson[i].Files.Files[j] = fileT{
				Type:     f.Type,
				Language: language,
				Original: original,
				Path:     fmt.Sprintf("%s%s", consts.CDN, f.UID),
				Size:     size,
				Length:   convertDuration(unit.Duration),
				Title:    language + " " + size,
			}
		}
	}

	c.XML(http.StatusOK, lessons)
}

func FeedPodcast(c *gin.Context) {
	var config feedConfig
	(&config).getConfig(c)

	db := c.MustGet("MDB_DB").(*sql.DB)
	cur := ContentUnitsRequest{
		ListRequest: ListRequest{
			BaseRequest: BaseRequest{
				Language: config.Lang,
			},
			StartIndex: 1,
			StopIndex:  150,
			OrderBy:    "created_at desc",
		},
		ContentTypesFilter: ContentTypesFilter{
			ContentTypes: []string{consts.CT_LESSON_PART},
		},
	}

	items, herr := handleContentUnits(db, cur)
	if herr != nil {
		herr.Abort(c)
		return
	}
	cuids := make([]int64, len(items.ContentUnits))
	for idx, cu := range items.ContentUnits {
		id, err := mapCU2ID(cu.ID, db, c)
		if err != nil {
			if err == sql.ErrNoRows {
				// empty feed
			} else {
				NewInternalError(err).Abort(c)
			}
			return
		}
		cuids[idx] = id
	}

	fileMap, err := loadCUFiles(db, cuids)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	feed := &feeds.Feed{
		Title:          "שיעור הקבלה היומי",
		Link:           getHref("/feeds/podcast.rss?DLANG="+config.DLANG, c),
		Description:    "כאן תקבלו עדכונים יומיים של שיעורי קבלה. התכנים מבוססים על מקורות הקבלה האותנטיים בלבד",
		Updated:        time.Now(),
		Copyright:      copyright,
		ItunesCategory: &feeds.ItunesCategory{Text: "Spirituality"},
		ItunesImage:    &feeds.ItunesImage{Href: getHref("/cover_podcast.jpg", c)},
		Author:         "info@kab.co.il",
		Items:          make([]*feeds.Item, 0),
	}

	var validFild = regexp.MustCompile("kitei-makor|lelo-mikud")
	for idx, cu := range items.ContentUnits {
		files, ok := fileMap[cuids[idx]]
		if !ok {
			NewInternalError(errors.Errorf("Illegal state: unit %s not in file map", cu.ID)).Abort(c)
			return
		}
		for _, file := range files {
			if file.MimeType.String != consts.MEDIA_MP3 || file.Language.String != config.Lang ||
				validFild.MatchString(file.Name) {
				continue
			}

			// TODO: change title and description
			url := fmt.Sprintf("%s%s", consts.CDN, file.UID)
			feed.Items = append(feed.Items, &feeds.Item{
				Author: "info@kab.co.il",
				Title:  cu.Name,
				Description: &feeds.Description{
					Text: "<div>" + file.Name + "; " + file.CreatedAt.Format("Mon, Jan _2 15:04:05 2006") + " </div>",
				},
				Guid: url,
				Link: url,
				Enclosure: &feeds.Enclosure{
					Url:    url,
					Length: file.Size,
					Type:   consts.MEDIA_MP3,
				},
				Created: cu.FilmDate.Time,
			})
		}
	}

	createFeed(feed, config.DLANG, true, c)
}

func FeedMorningLesson(c *gin.Context) {
	var err error
	var config feedConfig
	(&config).getConfig(c)
	t := T[config.DLANG]

	feed := &feeds.Feed{
		Title:       "Kabbalah Media Morning Lesson",
		Link:        getHref("/feeds/morning_lesson.rss?DLANG="+config.DLANG, c),
		Description: "The last lesson from Kabbalamedia Archive",
		Updated:     time.Now(),
		Copyright:   copyright,
	}

	db := c.MustGet("MDB_DB").(*sql.DB)
	lessonParts, herr := handleLatestLesson(db, BaseRequest{Language: config.Lang}, true)
	if herr != nil {
		herr.Abort(c)
	}

	cuids := make([]int64, len(lessonParts.ContentUnits))
	for idx, cu := range lessonParts.ContentUnits {
		id, err := mapCU2ID(cu.ID, db, c)
		if err != nil {
			if err == sql.ErrNoRows {
				// empty feed
			} else {
				NewInternalError(err).Abort(c)
			}
			return
		}
		cuids[idx] = id
	}
	fileMap, err := loadCUFiles(db, cuids)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	listen := "<h4>" + t.Playlist + "</h4>"
	download := "<h4>" + t.Download + "</h4>"

	for idx, cu := range lessonParts.ContentUnits {
		files, ok := fileMap[cuids[idx]]
		if !ok {
			NewInternalError(errors.Errorf("Illegal state: unit %s not in file map", cu.ID)).Abort(c)
			return
		}
		video := showAsset(config.Lang, consts.MEDIA_MP4, files, cu.Duration, t.Video+" mp4")
		audio := showAsset(config.Lang, consts.MEDIA_MP3, files, cu.Duration, t.Audio+" mp3")

		listen += "<div class='title'>" + cu.Name + "</div>" + video + audio
		download += "<div class='title'>" + cu.Name + "</div>" + video + audio
	}
	link := "https://archive.kbb1.com/ru/lessons/cu/" + lessonParts.ID
	feed.Items = []*feeds.Item{
		{
			Title:       t.LessonFrom + " " + lessonParts.FilmDate.Format("02.01.2006"),
			Guid:        link,
			Link:        link,
			Description: &feeds.Description{Text: listen + download},
			Created:     lessonParts.FilmDate.Time,
		},
	}

	createFeed(feed, config.DLANG, false, c)
}

func FeedRssPhp(c *gin.Context) {
	//var err error
	var config feedConfig
	(&config).getConfig(c)

	feed := &feeds.Feed{
		Title:       "Bnei-Baruch Kabbalahmedia MP3 Podcast",
		Link:        getHref("/rss.php?DLANG="+config.DLANG, c),
		Description: "Here you will find all the latest Kabbalah articles, videos, audio, news, features, Bnei Baruch website updates and content additions.",
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
			OrderBy:    "(properties->>'film_date')::date desc, created_at desc",
		},
		ContentTypesFilter: ContentTypesFilter{
			ContentTypes: []string{consts.CT_LESSON_PART},
		},
	}

	items, herr := handleContentUnits(db, cur)
	if herr != nil {
		herr.Abort(c)
		return
	}
	cuids := make([]int64, len(items.ContentUnits))
	for idx, cu := range items.ContentUnits {
		id, err := mapCU2ID(cu.ID, db, c)
		if err != nil {
			if err == sql.ErrNoRows {
				// empty feed
			} else {
				NewInternalError(err).Abort(c)
			}
			return
		}
		cuids[idx] = id
	}

	fileMap, err := loadCUFiles(db, cuids)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	feed.Items = make([]*feeds.Item, 0)
	for idx, cu := range items.ContentUnits {
		files, ok := fileMap[cuids[idx]]
		if !ok {
			NewInternalError(errors.Errorf("Illegal state: unit %s not in file map", cu.ID)).Abort(c)
			return
		}
		for _, file := range files {
			if file.MimeType.String != consts.MEDIA_MP4 {
				continue
			}

			url := fmt.Sprintf("%s%s", consts.CDN, file.UID)
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
					Type:   consts.MEDIA_MP4,
				},
				Created: cu.FilmDate.Time,
			})
		}
	}

	createFeed(feed, config.DLANG, false, c)
}

func showAsset(language string, mimeType string, files []*mdbmodels.File, duration float64, name string) string {
	for _, file := range files {
		if file.MimeType.String == mimeType && file.Language.String == language {
			size := convertSizeToMb(file.Size)
			var title string
			if duration == 0 {
				title = fmt.Sprintf("mp4&nbsp;|&nbsp;%.2fMb", size)
			} else {
				title = fmt.Sprintf("mp4&nbsp;|&nbsp;%.2fMb&nbsp;|&nbsp;%s", size, convertDuration(duration))
			}
			return fmt.Sprintf(
				`<a href="%s%s" title="%s">%s</a>`,
				consts.CDN, file.UID, title, name)
		}
	}

	return ""
}

func buildHtmlFromFile(language string, mimeType string, files []*mdbmodels.File, duration float64) string {
	for _, file := range files {
		if file.MimeType.String == mimeType && file.Language.String == language {
			size := convertSizeToMb(file.Size)
			var title string
			if duration == 0 {
				title = fmt.Sprintf("mp4&nbsp;|&nbsp;%.2fMb", size)
			} else {
				title = fmt.Sprintf("mp4&nbsp;|&nbsp;%.2fMb&nbsp;|&nbsp;%s", size, convertDuration(duration))
			}
			return fmt.Sprintf(
				`<a href="%s%s" title="%s">Открыть</a> | <a href="%s%s" title="%s">Скачать</a>`,
				consts.CDN, file.UID, title, consts.CDN, file.UID, title)
		}
	}

	return "N/A"
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
}
