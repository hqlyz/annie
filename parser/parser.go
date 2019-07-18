package parser

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/hqlyz/annie/myconfig"

	"github.com/PuerkitoBio/goquery"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

// SearchVideoData - the data structure of video info
type SearchVideoData struct {
	Title string `json:"title"`
	Img   string `json:"img"`
	URL   string `json:"url"`
	Dur   string `json:"dur"`
}

// GetDoc return Document object of the HTML string
func GetDoc(html string) (*goquery.Document, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// GetImages find the img with a given class name
func GetImages(
	url, html, imgClass string, urlHandler func(string) string, config myconfig.Config) (string, []downloader.URL, error) {
	var err error
	doc, err := GetDoc(html)
	if err != nil {
		return "", nil, err
	}
	title := Title(doc)
	urls := make([]downloader.URL, 0)
	urlData := downloader.URL{}
	doc.Find(fmt.Sprintf("img[class=\"%s\"]", imgClass)).Each(
		func(i int, s *goquery.Selection) {
			urlData.URL, _ = s.Attr("src")
			if urlHandler != nil {
				// Handle URL as needed
				urlData.URL = urlHandler(urlData.URL)
			}
			urlData.Size, err = request.Size(urlData.URL, url, config)
			if err != nil {
				return
			}
			_, urlData.Ext, err = utils.GetNameAndExt(urlData.URL, config)
			if err != nil {
				return
			}
			urls = append(urls, urlData)
		},
	)
	if err != nil {
		return "", nil, err
	}
	return title, urls, nil
}

// Title get title
func Title(doc *goquery.Document) string {
	var title string
	title = strings.Replace(
		strings.TrimSpace(doc.Find("h1").First().Text()), "\n", "", -1,
	)
	if title == "" {
		// Bilibili: Some movie page got no h1 tag
		title, _ = doc.Find("meta[property=\"og:title\"]").Attr("content")
	}
	if title == "" {
		title = doc.Find("title").Text()
	}
	return title
}

// GetSearchVideosInfo - get videos info with google search
func GetSearchVideosInfo(keyword string, config myconfig.Config) []SearchVideoData {
	searchURL := "https://www.google.com/search?tbm=vid&q=" + url.QueryEscape(keyword)
	html, err := request.Get(searchURL, "", myconfig.FakeHeaders, config)
	if err != nil {
		return nil
	}
	ioutil.WriteFile("search_html.html", []byte(html), 0644)
	titles := utils.MatchAll(html, `<h3 class="LC20lb">(.+?)</h3>`)
	urls := utils.MatchAll(html, `<div class="r"><a href="(.+?)"`)
	imgs1 := utils.MatchAll(html, `var s='(.+?)';var ii=\['vidthumb`)
	imgs2 := utils.MatchAll(html, `"vidthumb.":"(.+?)"`)
	imgs := append(imgs1, imgs2...)
	durs := utils.MatchAll(html, `<span class="vdur[^"]*">&#9654;&nbsp;([^<]+)<`)
	var (
		searchVideoData []SearchVideoData
		tempURL         string
		dur             string
		img             string
		title           string
	)
	for key := range urls {
		tempURL, err = url.QueryUnescape(urls[key][1])
		if err != nil {
			continue
		}
		if !isInArray(myconfig.SupportDomain, utils.Domain(tempURL)) {
			continue
		}
		if key < len(titles) {
			title = titles[key][1]
		} else {
			title = ""
		}
		if key < len(durs) {
			dur, err = url.QueryUnescape(durs[key][1])
			if err != nil {
				continue
			}
		} else {
			dur = "0"
		}
		if key < len(imgs) {
			img = imgs[key][1]
			img = strings.Replace(img, `\u003d`, "=", -1)
			img = strings.Replace(img, `\x3d`, "=", -1)
		} else {
			img = ""
		}
		item := SearchVideoData{
			Img:   img,
			URL:   tempURL,
			Dur:   dur,
			Title: title,
		}
		searchVideoData = append(searchVideoData, item)
	}
	return searchVideoData
}

func isInArray(arr []string, str string) bool {
	for _, s := range arr {
		if s == str {
			return true
		}
	}
	return false
}
