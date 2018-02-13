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

func FeedRusZohar(c *gin.Context) {
	var err error

	href := utils.ResolveScheme(c) + "://" + utils.ResolveHost(c) + "/feeds/rus_zohar.rss"
	copyright := fmt.Sprintf("Bnei-Baruch Copyright 2008-%d", time.Now().Year())
	feed := &feeds.Feed{
		Title:       "Kabbalah Media Zohar Lesson",
		Link:        &feeds.Link{Href: href},
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
	xu, err := mdbmodels.ContentUnits(db, qm.Where("uid = ?", cu.ID)).One()
	if err != nil {
		if err == sql.ErrNoRows {
			// empty feed
		} else {
			NewInternalError(err).Abort(c)
		}
		return
	}
	cuids := make([]int64, 1)
	cuids[0] = xu.ID
	fileMap, err := loadCUFiles(db, cuids)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}
	files, ok := fileMap[xu.ID]
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

	createFeed(feed, href, c)
}

func createFeed(feed *feeds.Feed, href string, c *gin.Context) {
	rss := &feeds.Rss{Feed: feed}
	rssFeed := rss.RssFeed()
	rssFeed.Language = "RUS"
	rssFeed.AtomLink = &feeds.RssAtomLink{
		Href: href,
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

func FeedRusForLaitmanRu(c *gin.Context) {
	//var err error
	//
	//href := utils.ResolveScheme(c) + "://" + utils.ResolveHost(c) + "/feeds/rus_for_laitman_ru.rss"
	//feed := &feeds.Feed{
	//	Title:       "Kabbalah Media Morning Lesson",
	//	Link:        &feeds.Link{Href: href},
	//	Description: "The last lesson from Kabbalamedia Archive",
	//	Updated:     time.Now(),
	//	Copyright:   "Bnei-Baruch Copyright 2008-" + time.Now().Year(),
	//}
	//
	//db := c.MustGet("MDB_DB").(*sql.DB)
}
