package api

import (
	"time"
	"database/sql"
	"net/http"
	"fmt"

	"gopkg.in/gin-gonic/gin.v1"
	"github.com/volatiletech/sqlboiler/queries/qm"

	"github.com/Bnei-Baruch/feeds"
	"github.com/Bnei-Baruch/archive-backend/utils"
	"github.com/Bnei-Baruch/archive-backend/mdb/models"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/pkg/errors"
)

var copyright = fmt.Sprintf("Bnei-Baruch Copyright 2008-%d", time.Now().Year())

func FeedRusZohar(c *gin.Context) {
	var err error

	feed := &feeds.Feed{
		Title:       "Kabbalah Media Zohar Lesson",
		Link:        &feeds.Link{Href: getHref("rus_zohar.rss", c)},
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

	feed.Items = []*feeds.Item{
		{
			Title: "Урок по Книге Зоар, " + cu.FilmDate.Format("02.01.2006"),
			Id:    cu.ID,
			Link:  &feeds.Link{Href: "https://archive.kbb1.com/ru/lessons/cu/" + cu.ID},
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
	rss := &feeds.Rss{Feed: feed}
	rssFeed := rss.RssFeed()
	rssFeed.Language = language
	rssFeed.AtomLink = &feeds.RssAtomLink{
		Href: feed.Link.Href,
		Rel:  "self",
		Type: "application/rss+xml",
	}
	content, err := feeds.ToXML(rssFeed)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	c.String(http.StatusOK, content)
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
				`<a href="%s/%s" title="%s">Открыть</a> | <a href="%s/%s" title="%s">Скачать</a>`,
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
	return utils.ResolveScheme(c) + "://" + utils.ResolveHost(c) + "/feeds/" + href
}

func FeedRusForLaitmanRu(c *gin.Context) {
	var err error

	feed := &feeds.Feed{
		Title:       "Kabbalah Media Morning Lesson",
		Link:        &feeds.Link{Href: getHref("rus_for_laitman_ru.rss", c)},
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

		feed.Items[idx] = &feeds.Item{
			Title: "Утренний урок " + cu.FilmDate.Format("02.01.2006"),
			Id:    cu.ID,
			Link:  &feeds.Link{Href: "https://archive.kbb1.com/ru/lessons/cu/" + cu.ID},
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

type Dlang struct {
	DLANG string
	Lang  string
}

type Translation struct {
	LessonFrom string
	Playlist   string
	Download   string
	Video      string
	Audio      string
}

var T = map[string]Translation{
	"ENG": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"HEB": {"שיעור מתאריך", "רשימת השמעה:", "הורדת השיעור:", "וידאו", "אודיו"},
	"RUS": {"Урок от", "Плейлист:", "Скачать:", "видео", "аудио"},
	"SPA": {"Clase de", "Lista de reproducción:", "Descargar:", "Video", "Audio"},
	"ITA": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"GER": {"Unterricht von", "Spielliste:", "Downloaden:", "Video", "Audio"},
	"DUT": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"FRE": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"POR": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"TRK": {"Dersler", "Çalma listesi:", "Yükle:", "Video", "ses"},
	"POL": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"ARB": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"HUN": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"FIN": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"LIT": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"JPN": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"BUL": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"GEO": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"NOR": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"SWE": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"HRV": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"CHN": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"FAR": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"RON": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"HIN": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"UKR": {"Урок від", "Плейлист:", "Завантажити:", "відео", "аудіо"},
	"MKD": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"SLV": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"LAV": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"SLK": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
	"CZE": {"Lesson from", "Playlist:", "Download:", "Video", "Audio"},
}

func FeedMorningLesson(c *gin.Context) {
	var err error
	var dlang Dlang
	if c.Bind(&dlang) != nil {
		return
	}
	if dlang.DLANG == "" {
		dlang = Dlang{DLANG: "ENG", Lang: consts.LANG_ENGLISH}
	} else {
		dlang.Lang = consts.CODE2LANG[dlang.DLANG]
		if dlang.Lang == "" {
			dlang = Dlang{DLANG: "ENG", Lang: consts.LANG_ENGLISH}
		}
	}
	t := T[dlang.DLANG]

	feed := &feeds.Feed{
		Title:       "Kabbalah Media Morning Lesson",
		Link:        &feeds.Link{Href: getHref("morning_lesson.rss?DLANG="+dlang.DLANG, c)},
		Description: "The last lesson from Kabbalamedia Archive",
		Updated:     time.Now(),
		Copyright:   copyright,
	}

	db := c.MustGet("MDB_DB").(*sql.DB)
	lessonParts, herr := handleLatestLesson(db, BaseRequest{Language: dlang.Lang}, true)
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
		video := showAsset(dlang.Lang, consts.MEDIA_MP4, files, cu.Duration, t.Video + " mp4")
		audio := showAsset(dlang.Lang, consts.MEDIA_MP3, files, cu.Duration, t.Audio + " mp3")

		listen += "<div class='title'>" + cu.Name + "</div>" + video + audio
		download += "<div class='title'>" + cu.Name + "</div>" + video + audio
	}
	feed.Items = []*feeds.Item{
		{
			Title: t.LessonFrom + " " + lessonParts.FilmDate.Format("02.01.2006"),
			Id:    lessonParts.ID,
			Link:  &feeds.Link{Href: "https://archive.kbb1.com/ru/lessons/cu/" + lessonParts.ID},
			Description: &feeds.Description{Text: listen + download},
			Created: lessonParts.FilmDate.Time,
		},
	}

	createFeed(feed, dlang.DLANG, c)
}

func showAsset(language string, mimeType string, files []*mdbmodels.File, duration float64, name string) (string) {
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
				`<a href="%s/%s" title="%s">%s</a>`,
				consts.CDN, file.UID, title, name)
		}
	}

	return ""
}