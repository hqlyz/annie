package vimeo

import (
	"encoding/json"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/hqlyz/annie/myconfig"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

type vimeoProgressive struct {
	Profile int    `json:"profile"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Quality string `json:"quality"`
	URL     string `json:"url"`
}

type vimeoFiles struct {
	Progressive []vimeoProgressive `json:"progressive"`
}

type vimeoRequest struct {
	Files vimeoFiles `json:"files"`
}

type vimeoVideo struct {
	Title    string    `json:"title"`
	Duration int       `json:"duration"`
	Thumbs   thumbData `json:"thumbs"`
}

type thumbData struct {
	T640 string `json:"640"`
}

type vimeo struct {
	Request vimeoRequest `json:"request"`
	Video   vimeoVideo   `json:"video"`
}

// Extract is the main function for extracting data
func Extract(url string, config myconfig.Config) ([]downloader.Data, error) {
	var (
		html, vid string
		err       error
	)
	if strings.Contains(url, "player.vimeo.com") {
		html, err = request.Get(url, url, nil, config)
		if err != nil {
			return downloader.EmptyList, err
		}
	} else {
		vid = utils.MatchOneOf(url, `vimeo\.com/(\d+)`)[1]
		html, err = request.Get("https://player.vimeo.com/video/"+vid, url, nil, config)
		if err != nil {
			return downloader.EmptyList, err
		}
	}
	ioutil.WriteFile("E:/vimeo.html", []byte(html), 0644)
	jsonString := utils.MatchOneOf(html, `var \w+\s?=\s?({.+?});`)[1]
	var vimeoData vimeo
	json.Unmarshal([]byte(jsonString), &vimeoData)
	streams := map[string]downloader.Stream{}
	var size int64
	var urlData downloader.URL
	for _, video := range vimeoData.Request.Files.Progressive {
		size, err = request.Size(video.URL, url, config)
		if err != nil {
			return downloader.EmptyList, err
		}
		urlData = downloader.URL{
			URL:  video.URL,
			Size: size,
			Ext:  "mp4",
		}
		streams[strconv.Itoa(video.Profile)] = downloader.Stream{
			URLs:    []downloader.URL{urlData},
			Size:    size,
			Quality: video.Quality,
		}
	}

	return []downloader.Data{
		{
			Site:      "Vimeo vimeo.com",
			Title:     vimeoData.Video.Title,
			Type:      "video",
			Streams:   streams,
			URL:       url,
			Thumbnail: vimeoData.Video.Thumbs.T640,
			Length:    strconv.Itoa(vimeoData.Video.Duration),
		},
	}, nil
}
