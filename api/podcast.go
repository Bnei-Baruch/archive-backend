package api

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"time"

	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/consts"
)

type podcastFeedXml struct {
	XMLName       xml.Name `xml:"rss"`
	Version       string   `xml:"version,attr"`
	XmlnsItunes   string   `xml:"xmlns:itunes,attr"`
	XmlnsAtom     string   `xml:"xmlns:atom,attr"`
	XmlnsRawvoice string   `xml:"xmlns:rawvoice,attr,omitempty"`

	Channel *podcastChannel
}

type podcastChannel struct {
	XMLName         xml.Name `xml:"channel"`
	Title           string   `xml:"title"`       // required
	Link            string   `xml:"link"`        // required
	Description     string   `xml:"description"` // required
	Image           *podcastImage
	Language        string `xml:"language"`
	Copyright       string `xml:"copyright"`
	PodcastAtomLink *podcastAtomLink
	LastBuildDate   string `xml:"lastBuildDate"`
	Author          string `xml:"itunes:author"`
	Summary         string `xml:"itunes:summary"`
	Subtitle        string `xml:"itunes:subtitle,omitempty"`
	Owner           *podcastOwner
	Explicit        string `xml:"itunes:explicit"`
	Keywords        string `xml:"itunes:keywords"`
	ItunesImage     *itunesImage
	Category        *podcastCategory
	PubDate         string `xml:"pubDate"`
	Items           []*podcastItem
}

type podcastItem struct {
	XMLName     xml.Name `xml:"item"`
	Title       string   `xml:"title"`             // required
	Link        string   `xml:"link"`              // required
	PubDate     string   `xml:"pubDate,omitempty"` // created or updated
	Description string   `xml:"description"`       // required
	Enclosure   *podcastEnclosure
	Guid        string `xml:"guid"`
	Duration    string `xml:"itunes:duration"`
	Summary     string `xml:"itunes:summary"`
	Image       *itunesImage
	Keywords    string `xml:"itunes:keywords,omitempty"`
	Explicit    string `xml:"itunes:explicit"`
}

type podcastEnclosure struct {
	XMLName xml.Name `xml:"enclosure"`
	Url     string   `xml:"url,attr"`
	Length  int64    `xml:"length,attr"`
	Type    string   `xml:"type,attr"`
}

type podcastCategory struct {
	XMLName  xml.Name `xml:"itunes:category"`
	Text     string   `xml:"text,attr"`
	Category *podcastCategory
}

type podcastOwner struct {
	XMLName xml.Name `xml:"itunes:owner,omitempty"`
	Name    string   `xml:"itunes:name,omitempty"`
	Email   string   `xml:"itunes:email,omitempty"`
}

type podcastImage struct {
	XMLName xml.Name `xml:"image"`
	Url     string   `xml:"url"`
	Title   string   `xml:"title"`
	Link    string   `xml:"link"`
}

type itunesImage struct {
	XMLName xml.Name `xml:"itunes:image"`
	Href    string   `xml:"href,attr"`
}

type podcastAtomLink struct {
	XMLName xml.Name `xml:"atom:link"`
	Href    string   `xml:"href,attr"`
	Rel     string   `xml:"rel,attr"`
	Type    string   `xml:"type,attr"`
}

func (c *podcastChannel) CreateFeed() interface{} {
	return &podcastFeedXml{
		Version:     "2.0",
		XmlnsItunes: "http://www.itunes.com/dtds/podcast-1.0.dtd",
		XmlnsAtom:   "http://www.w3.org/2005/Atom",
		Channel:     c,
	}
}

func FeedPodcast(c *gin.Context) {
	var config feedConfig
	(&config).getConfig(c)

	type translation struct {
		Title       string
		Description string
		Keywords    string
		Author      string
	}
	var T = map[string]translation{
		"ENG": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"HEB": {Title: "שיעור הקבלה היומי", Description: "במשך אלפי שנים, היו המקובלים לומדים על בסיס יומי, למען התפתחותם הרוחנית הפרטית ולמען התקדמותה הרוחנית של האנושות. בימינו, ממשיכים את אותה המסורת קבוצת המקובלים ״בני ברוך״, הלומדים מדי יום מתוך כתבי הקבלה האותנטיים, לימודים המלווים בביאור והדרכה מפי הרב ד״ר מיכאל לייטמן.", Keywords: "קבלה,שיעור,רוחניות,אותנטי", Author: "ד״ר מיכאל לייטמן"},
		"RUS": {Title: "Ежедневный урок по каббале", Description: "На протяжении тысячелетий каббалисты учились каждый день, ради своего личного духовного возвышения, и ради духовного возвышения человечества. В наше время продолжает эту традицию каббалистическая группа \"Бней Барух\",  занимаясь ежедневно по подлинным каббалистическим источникам, под руководством ученого – каббалиста, основателя Международной академии каббалы, Михаэля Лайтмана.", Keywords: "каббала,уроки,духовность,аутентичная", Author: "Михаэль Лайтман"},
		"UKR": {Title: "Ежедневный урок по каббале", Description: "На протяжении тысячелетий каббалисты учились каждый день, ради своего личного духовного возвышения, и ради духовного возвышения человечества. В наше время продолжает эту традицию каббалистическая группа \"Бней Барух\",  занимаясь ежедневно по подлинным каббалистическим источникам, под руководством ученого – каббалиста, основателя Международной академии каббалы, Михаэля Лайтмана.", Keywords: "каббала,уроки,духовность,аутентичная", Author: "Михаэль Лайтман"},
		"SPA": {Title: "Daily Kabbalah Lesson", Description: "Durante miles de años, los cabalistas se consagraron a estudiar día tras día para alcanzar el progreso espiritual de la humanidad y el suyo propio. En el Instituto Bnei Baruj para la Educación y la Investigación de la Cabalá continuamos con esta tradición en el mundo globalizado de hoy, estudiando diariamente las fuentes auténticas cabalísticas, enriquecidas con los comentarios del Rav doctor Michael Laitman", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"ITA": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"GER": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"DUT": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"FRE": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"POR": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"TRK": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"POL": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"ARB": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"HUN": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"FIN": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"LIT": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"JPN": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"BUL": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"GEO": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"NOR": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"SWE": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"HRV": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"CHN": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"PER": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"RON": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"HIN": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"MKD": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"SLV": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"LAV": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"SLK": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
		"CZE": {Title: "Daily Kabbalah Lesson", Description: "For thousands of years, Kabbalists have been studying on a daily basis for their and humanity's spiritual progress. Continuing this tradition into today's globally connected world, the Bnei Baruch Kabbalah Education & Research Institute, studies daily from authentic Kabbalistic sources, with commentary by Dr. Michael Laitman.", Keywords: "kabbalah,lessons,spirituality,authentic", Author: "Dr. Michael Laitman"},
	}

	t := T[config.DLANG]
	title := t.Title
	description := t.Description
	href := "https://old.kabbalahmedia.info/cover_podcast.jpg"
	link := getHref("/feeds/podcast.rss?DLANG="+config.DLANG, c)

	channel := &podcastChannel{
		Title:           title,
		Link:            "https://www.kabbalahmedia.info/",
		Description:     description,
		Image:           &podcastImage{Url: href, Title: title, Link: link},
		Language:        config.Lang,
		Copyright:       copyright,
		PodcastAtomLink: &podcastAtomLink{Href: link, Rel: "self", Type: "application/rss+xml"},
		LastBuildDate:   time.Now().Format(time.RFC1123), // TODO: get a newest created_at of files
		Author:          t.Author,
		Summary:         description,
		Subtitle:        "",
		Owner:           &podcastOwner{Name: "Bnei Baruch Association", Email: "info@kab.co.il"},
		Explicit:        "no",
		Keywords:        t.Keywords,
		ItunesImage:     &itunesImage{Href: href},
		Category:        &podcastCategory{Text: "Religion & Spirituality", Category: &podcastCategory{Text: "Spirituality"}},
		PubDate:         time.Now().Format(time.RFC1123),

		Items: make([]*podcastItem, 0),
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
	}

	//DF=[A]/V
	var mediaTypes []string
	if config.DF == "V" {
		mediaTypes = []string{consts.MEDIA_MP4}
	} else if config.DF == "A" {
		mediaTypes = []string{consts.MEDIA_MP3a, consts.MEDIA_MP3b}
	} else {
		mediaTypes = []string{consts.MEDIA_MP4, consts.MEDIA_MP3a, consts.MEDIA_MP3b}
	}
	languages := []string{config.Lang}
	item, err := handleContentUnitsFull(db, cur, mediaTypes, languages)
	if err != nil {
		if err == sql.ErrNoRows {
			c.XML(http.StatusOK, channel.CreateFeed())
		} else {
			NewInternalError(err).Abort(c)
		}
		return
	}

	var nameToIgnore = regexp.MustCompile("kitei-makor|lelo-mikud")
	for _, cu := range item.ContentUnits {
		files := cu.Files
		for _, file := range files {
			if nameToIgnore.MatchString(file.Name) {
				continue
			}

			// TODO: change title and description
			url := fmt.Sprintf("%s%s%s", consts.CDN, file.ID, filepath.Ext(file.Name))
			description := cu.Description
			if description == "" {
				description = cu.Name
			}
			channel.Items = append(channel.Items, &podcastItem{
				Title:       file.CreatedAt.Format(time.RFC822) + "; " + cu.Name,
				Link:        url,
				PubDate:     file.CreatedAt.Format(time.RFC822),
				Description: description,
				Enclosure: &podcastEnclosure{
					Url:    url,
					Length: file.Size,
					Type:   consts.MEDIA_MP3,
				},
				Guid:     url,
				Duration: convertDuration(cu.Duration),
				Summary:  description,
				Image:    &itunesImage{Href: href},
				Keywords: t.Keywords,
				Explicit: "no",
			})
		}
	}

	feed := channel.CreateFeed()
	feedXml, err := xml.Marshal(feed)
	payload := []byte(xml.Header + string(feedXml))
	c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(payload))
}
