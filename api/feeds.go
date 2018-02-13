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
)

func FeedRusZohar(c *gin.Context) {
	var err error

	db := c.MustGet("MDB_DB").(*sql.DB)

	cur := ContentUnitsRequest{
		ListRequest{
			BaseRequest: BaseRequest{
				Language: consts.LANG_RUSSIAN,
			},
			PageNumber: 1,
			PageSize:   1,
			OrderBy:    "(properties->>'film_date')::date desc, created_at desc",
		},
		IDsFilter{},
		ContentTypesFilter{
			ContentTypes: []string{consts.CT_LESSON_PART},
		},
		DateRangeFilter{},
		SourcesFilter{Sources: []string{"AwGBQX2L"}}, // Zohar
		TagsFilter{},
		GenresProgramsFilter{},
		CollectionsFilter{},
		PublishersFilter{},
	}
	item, herr := handleContentUnits(db, cur)
	if herr != nil {
		c.AbortWithError(400, herr.Err)
		return
	}
	cu := item.ContentUnits[0]
	xu, err := mdbmodels.ContentUnits(db,
		SECURE_PUBLISHED_MOD,
		qm.Where("uid = ?", cu.ID),
		qm.Load(
			"CollectionsContentUnits",
			"CollectionsContentUnits.Collection",
		)).
		One()
	if err != nil {
		c.AbortWithError(400, err)
		return
	}
	cuids := make([]int64, 1)
	cuids[0] = xu.ID
	fileMap, err := loadCUFiles(db, cuids)
	if err != nil {
		c.AbortWithError(400, err)
		return
	}
	files, ok := fileMap[xu.ID]
	if !ok {
		c.AbortWithError(400, err)
		return
	}

	href := utils.ResolveScheme(c) + "://" + utils.ResolveHost(c) + "/feeds/rus_zohar.rss"
	feed := &feeds.Feed{
		Title:       "Kabbalah Media Zohar Lesson",
		Link:        &feeds.Link{Href: href},
		Description: "The evening Zohar lesson from Kabbalahmedia Archive",
		Updated:     time.Now(),
		Copyright:   "Bnei-Baruch Copyright 2008-2018",
	}

	videoRus := buildHtmlFromFile(consts.LANG_RUSSIAN, consts.MEDIA_MP4, files, cu.Duration)
	audioRus := buildHtmlFromFile(consts.LANG_RUSSIAN, consts.MEDIA_MP3, files, cu.Duration)
	videoHeb := buildHtmlFromFile(consts.LANG_HEBREW, consts.MEDIA_MP4, files, cu.Duration)
	audioHeb := buildHtmlFromFile(consts.LANG_HEBREW, consts.MEDIA_MP3, files, cu.Duration)

	feed.Items = []*feeds.Item{
		{
			Title: "Урок по Книге Зоар, " + cu.FilmDate.Format("02.01.2006"),
			Id:    cu.ID,
			Link:  &feeds.Link{Href: "https://archive.kbb1.com/lessons/cu/" + cu.ID},
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
		c.AbortWithError(400, err)
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
	return float64(size) / 1024 / 1025
}

func convertDuration(duration float64) string {
	return time.Unix(int64(duration), 0).UTC().Format("15:04:05")
}
