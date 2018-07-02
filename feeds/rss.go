package feeds

import (
	"encoding/xml"
	"time"
	"strings"
)

type AtomLink struct {
	XMLName xml.Name `xml:"atom:link"`
	Href    string   `xml:"href,attr"`
	Rel     string   `xml:"rel,attr"`
	Type    string   `xml:"type,attr"`
}

type ItunesCategory struct {
	XMLName xml.Name `xml:"itunes:category"`
	Text    string   `xml:"text,attr"`
}

type ItunesImage struct {
	XMLName xml.Name `xml:"itunes:image"`
	Href    string   `xml:"href,attr"`
}

type Description struct {
	XMLName xml.Name `xml:"description"`
	Text    string   `xml:",cdata"`
}

type Enclosure struct {
	XMLName xml.Name `xml:"enclosure"`
	Url     string   `xml:"url,attr"`
	Length  int64    `xml:"length,attr"`
	Type    string   `xml:"type,attr"`
}

type Feed struct {
	Title          string    `xml:"title"`       // required
	Link           string    `xml:"link"`        // required
	Description    string    `xml:"description"` // required
	Language       string    `xml:"language"`
	Copyright      string    `xml:"copyright"`
	Author         string    `xml:"author,omitempty"`
	PubDate        string    `xml:"pubDate,omitempty"`
	Created        time.Time `xml:"-"`
	Updated        time.Time `xml:"-"`
	ItunesCategory *ItunesCategory
	ItunesImage    *ItunesImage
	Items          []*Item
}

type Channel struct {
	XMLName       xml.Name `xml:"channel"`
	AtomLink      *AtomLink
	Ttl           int      `xml:"ttl,omitempty"`
	LastBuildDate string   `xml:"lastBuildDate,omitempty"`
	*Feed
}

type Item struct {
	XMLName      xml.Name     `xml:"item"`
	Title        string       `xml:"title"`       // required
	Link         string       `xml:"link"`        // required
	Description  *Description `xml:"description"` // required
	Author       string       `xml:"dc:creator,omitempty"`
	SimpleAuthor string       `xml:"author,omitempty"`
	Category     string       `xml:"category,omitempty"`
	Comments     string       `xml:"comments,omitempty"`
	Enclosure    *Enclosure
	Guid         string       `xml:"guid,omitempty"`
	Source       string       `xml:"source,omitempty"`
	PubDate      string       `xml:"pubDate,omitempty"` // created or updated
	Created      time.Time    `xml:"-"`
}

func (r *Feed) RssFeed() (channel *Channel) {
	if r.PubDate == "" {
		r.PubDate = anyTimeFormat(time.RFC1123, r.Created, time.Now())
	}
	for _, item := range r.Items {
		item.PubDate = anyTimeFormat(time.RFC1123, r.Created, time.Now())
	}
	channel = &Channel{
		AtomLink: &AtomLink{
			Href: r.Link,
			Rel:  "self",
			Type: "application/rss+xml",
		},
		Ttl:           600,
		LastBuildDate: anyTimeFormat(time.RFC1123, r.Created, r.Updated, time.Now()),
		Feed:          r,
	}
	return channel
}

// private wrapper around the RssFeed which gives us the <rss>..</rss> xml
type rssFeedXml struct {
	XMLName    xml.Name `xml:"rss"`
	Version    string   `xml:"version,attr"`
	XmlnsMedia string   `xml:"xmlns:media,attr"`
	XmlnsAtom  string   `xml:"xmlns:atom,attr"`
	Channel    *Channel
}

// return XML-ready object for a Channel
func (c *Channel) FeedXml(isItunes bool) interface{} {
	var media string
	if isItunes {
		media = "http://www.itunes.com/dtds/podcast-1.0.dtd"
	} else {
		media = "http://search.yahoo.com/mrss/"
	}
	return &rssFeedXml{
		Version:    "2.0",
		XmlnsAtom:  "http://www.w3.org/2005/Atom",
		XmlnsMedia: media,
		Channel:    c,
	}
}

// returns the first non-zero time formatted as a string or ""
func anyTimeFormat(format string, times ...time.Time) string {
	for _, t := range times {
		if !t.IsZero() {
			// Always return GMT time by converting to UTC and then replacing UTC with GMT in the output string (RSS doesn't allow UTC)
			timeFormatted := t.UTC().Format(format)
			return strings.Replace(timeFormatted, "UTC", "GMT", -1)
		}
	}
	return ""
}

// turn a feed object (either a Feed, AtomFeed, or RssFeed) into xml
// returns an error if xml marshaling fails
func (c *Channel) ToXML(isItunes bool) (string, error) {
	x := c.FeedXml(isItunes)
	data, err := xml.MarshalIndent(x, "", "  ")
	if err != nil {
		return "", err
	}
	// strip empty line from default xml header
	s := xml.Header[:len(xml.Header)-1] + string(data)
	return s, nil
}
