/*
  This is free and unencumbered software released into the public domain. For more
  information, see <http://unlicense.org/> or the accompanying UNLICENSE file.
*/

// Simple parser for RSS and Atom feeds
package feedparser

import (
	"code.google.com/p/go-charset/charset"
	_ "code.google.com/p/go-charset/data"
	"encoding/xml"
	"io"
	"strings"
	"time"
)

type Feed struct {
	Title    string
	Subtitle string
	Link     string
	Items    []*FeedItem
}

type Media struct {
	Url  string
	Size string
}

type FeedItem struct {
	Id          string
	Title       string
	Description string
	Link        string
	Image       string
	ImageSource string
	When        time.Time
	Enclosure   string
	Media       []Media
}

const feedTitle = "title"
const (
	atomNs  = "http://www.w3.org/2005/atom"
	mediaNs = "http://search.yahoo.com/mrss/"
	ytNs    = "http://gdata.youtube.com/schemas/2007"
)

const (
	rssChannel     = "channel"
	rssItem        = "item"
	rssLink        = "link"
	rssPubDate     = "pubdate"
	rssDescription = "description"
	rssId          = "guid"
)

const (
	atomSubtitle = "subtitle"
	atomFeed     = "feed"
	atomEntry    = "entry"
	atomLink     = "link"
	atomLinkHref = "href"
	atomUpdated  = "updated"
	atomSummary  = "summary"
	atomId       = "id"
)

const (
	mediaGroup     = "group"
	mediaThumbnail = "thumbnail"
)

const (
	attrUrl  = "url"
	attrName = "name"
)

const (
	levelFeed = iota
	levelPost
)

func parseTime(f, v string) time.Time {
	t, err := time.Parse(f, v)
	if err != nil || v == "" {
		return time.Now()
	}
	return t
}

func NewFeed(r io.Reader) (*Feed, error) {
	var ns string
	var tag string
	var atom bool
	var level int
	feed := &Feed{}
	item := &FeedItem{}
	item.Media = []Media{}
	parser := xml.NewDecoder(r)
	parser.Strict = false
	parser.CharsetReader = charset.NewReader
	linkOk := false
	var st xml.StartElement
	for {
		token, err := parser.Token()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			ns = strings.ToLower(t.Name.Space)
			tag = strings.ToLower(t.Name.Local)
			st = t
			switch {
			case tag == atomFeed:
				atom = true
				level = levelFeed
			case tag == rssChannel:
				atom = false
				level = levelFeed
			case (!atom && tag == rssItem) || (atom && tag == atomEntry):
				level = levelPost
				linkOk = false
				item = &FeedItem{When: time.Now()}
				item.Media = []Media{}

			case tag == "enclosure":
				for _, a := range t.Attr {
					if strings.ToLower(a.Name.Local) == "url" {
						item.Enclosure = a.Value
						item.Media = append(item.Media, Media{Url: a.Value})
						break
					}
				}

			case atom && tag == atomLink:
				if (level == levelPost) && linkOk {
					break
				}
				for _, a := range t.Attr {
					if strings.ToLower(a.Name.Local) == atomLinkHref {
						switch level {
						case levelFeed:
							feed.Link = a.Value
						case levelPost:
							item.Link = a.Value
						}
					}
					if (level == levelPost) && strings.ToLower(a.Name.Local) == "rel" {
						if a.Value == "alternate" {
							linkOk = true
						}
					}
				}

			case ns == mediaNs && tag == "content":
				if level != levelPost {
					break
				}
				ok := false
				for _, attr := range t.Attr {
					if (strings.ToLower(attr.Name.Local)) == "type" && strings.HasPrefix(strings.ToLower(attr.Value), "video") {
						ok = true
						break
					}
				}
				if ok {
					for _, attr := range t.Attr {
						if (strings.ToLower(attr.Name.Local)) == "url" {
							item.Media = append(item.Media, Media{Url: attr.Value})
							break
						}
					}
					for _, attr := range t.Attr {
						if (strings.ToLower(attr.Name.Local)) == "filesize" {
							item.Media[len(item.Media)-1].Size = attr.Value
							break
						}
					}
				}

			case ns == mediaNs && tag == mediaThumbnail:
				var url, name string
				for _, attr := range t.Attr {
					ns := strings.ToLower(attr.Name.Space)
					a := strings.ToLower(attr.Name.Local)
					switch {
					case a == attrUrl:
						url = attr.Value

					case ns == ytNs && a == attrName:
						name = attr.Value
					}
				}
				if url != "" && (item.Image == "" ||
					name == "sddefault" ||
					(name == "hqdefault" && item.ImageSource != "sddefault") ||
					(name == "mqdefault" && item.ImageSource != "sddefault" && item.ImageSource != "hqdefault") ||
					(name == "default " && item.ImageSource != "mqdefault" && item.ImageSource != "sddefault" && item.ImageSource != "hqdefault")) {
					item.Image = url
					item.ImageSource = name

				}
			}

		case xml.EndElement:
			e := strings.ToLower(t.Name.Local)
			if e == atomEntry || e == rssItem {
				if item.Id == "" {
					item.Id = item.Link
				}
				feed.Items = append(feed.Items, item)
			}
		case xml.CharData:
			text := string([]byte(t))
			if strings.TrimSpace(text) == "" {
				continue
			}
			switch level {
			case levelFeed:
				switch {
				case tag == feedTitle:
					if feed.Title == "" {
						feed.Title = text
					}
				case (!atom && tag == rssDescription) || (atom && tag == atomSubtitle):
					feed.Subtitle = text
				case !atom && tag == rssLink:
					feed.Link = text
				}
			case levelPost:
				switch {
				case (!atom && tag == rssId) || (atom && tag == atomId):
					item.Id = text
				case (ns == "" || ns == atomNs) && tag == feedTitle:
					item.Title = text
				case (!atom && tag == rssDescription) || (atom && tag == atomSummary):
					item.Description = text
				case !atom && tag == rssLink:
					if !linkOk {
						for _, a := range st.Attr {
							if strings.ToLower(a.Name.Local) == "rel" {
								if a.Value == "alternate" {
									linkOk = true
								}
							}
						}
						item.Link = text
					}
				case atom && tag == atomUpdated:
					var f string
					switch {
					case strings.HasSuffix(strings.ToUpper(text), "Z"):
						f = "2006-01-02T15:04:05Z"
					default:
						f = "2006-01-02T15:04:05-07:00"
					}
					item.When = parseTime(f, text)
				case !atom && tag == rssPubDate:
					var f string
					if strings.HasSuffix(strings.ToUpper(text), "T") {
						f = "Mon, 2 Jan 2006 15:04:05 MST"
					} else {
						f = "Mon, 2 Jan 2006 15:04:05 -0700"
					}
					item.When = parseTime(f, text)
				}

			}
		}
	}
	return feed, nil
}
