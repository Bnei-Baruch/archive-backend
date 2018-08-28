package api

import (
	"time"

	"gopkg.in/gin-gonic/gin.v1"

	"encoding/xml"
	"net/http"
	"database/sql"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"regexp"
	"fmt"
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

	title := "שיעור הקבלה היומי"
	href := "https://old.kabbalahmedia.info/cover_podcast.jpg"
	link := getHref("/feeds/podcast.rss?DLANG="+config.DLANG, c)
	description := "כאן תקבלו עדכונים יומיים של שיעורי קבלה. התכנים מבוססים על מקורות הקבלה האותנטיים בלבד"

	channel := &podcastChannel{
		Title:           title,
		Link:            "https://www.kabbalahmedia.info/",
		Description:     description,
		Image:           &podcastImage{Url: href, Title: title, Link: link},
		Language:        "he",
		Copyright:       copyright,
		PodcastAtomLink: &podcastAtomLink{Href: link, Rel: "self", Type: "application/rss+xml"},
		LastBuildDate:   time.Now().Format(time.RFC1123), // TODO: get a newest created_at of files
		Author:          "Dr. Michael Laitman",
		Summary:         description,
		Subtitle:        "",
		Owner:           &podcastOwner{Name: "Dr. Michael Laitman", Email: "info@kab.co.il"},
		Explicit:        "no",
		Keywords:        "קבלה,שיעור,מקור,אותנטי",
		ItunesImage:     &itunesImage{Href: href},
		Category:        &podcastCategory{Text: "Religion &amp; Spirituality", Category: &podcastCategory{Text: "Spirituality"}},
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

	item, herr := handleContentUnits(db, cur)
	if herr != nil {
		herr.Abort(c)
		return
	}
	cuids, err := mapCU2IDs(item.ContentUnits, db, c)
	if err != nil {
		if err == sql.ErrNoRows {
			c.XML(http.StatusOK, channel.CreateFeed())
		} else {
			NewInternalError(err).Abort(c)
		}
		return
	}

	mediaTypes := []string{consts.MEDIA_MP3a, consts.MEDIA_MP3b,}
	fileMap, err := loadCUFiles(db, cuids, mediaTypes, config.Lang)
	if err != nil {
		NewInternalError(err).Abort(c)
		return
	}

	var nameToIgnore = regexp.MustCompile("kitei-makor|lelo-mikud")
	for idx, cu := range item.ContentUnits {
		files, ok := fileMap[cuids[idx]]
		if !ok { // CU without files
			continue
		}
		for _, file := range files {
			if nameToIgnore.MatchString(file.Name) {
				continue
			}

			// TODO: change title and description
			url := fmt.Sprintf("%s%s", consts.CDN, file.UID)
			description := cu.Description
			if description == "" {
				description = cu.Name
			}
			channel.Items = append(channel.Items, &podcastItem{
				Title:       file.Name + "; " + file.CreatedAt.Format(time.RFC822),
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
				Keywords: "קבלה,שיעור,מקור,אותנטי",
				Explicit: "no",
			})
		}
	}

	feed := channel.CreateFeed()
	feedXml, err := xml.Marshal(feed)
	xml := []byte(xml.Header + string(feedXml))
	c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(xml))
}
