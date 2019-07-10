package pornhub

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/hqlyz/annie/myconfig"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

type pornhubData struct {
	Format   string      `json:"format"`
	Quality  interface{} `json:"quality"`
	VideoURL string      `json:"videoUrl"`
}

// Extract is the main function for extracting data
func Extract(url string, config myconfig.Config) ([]downloader.Data, error) {
	var (
		title    string
		err      error
		duration string
	)
	html, err := request.Get(url, url, nil, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	ioutil.WriteFile("ph_html.html", []byte(html), 0666)
	desc := utils.MatchOneOf(html, `<span class="inlineFree">(.+?)</span>`)
	if desc != nil {
		title = desc[1]
	} else {
		title = "pornhub video"
	}
	dur := utils.MatchOneOf(html, `meta property="video:duration" content="(.+?)" />`)
	if dur != nil {
		duration = dur[1]
	} else {
		duration = "0"
	}

	realURLs := utils.MatchOneOf(html, `"mediaDefinitions":(.+?),"isVertical"`)
	// ioutil.WriteFile("ph.txt", []byte(realURLs[1]), 0666)

	var pornhubs []pornhubData
	err = json.Unmarshal([]byte(realURLs[1]), &pornhubs)
	if err != nil {
		return downloader.EmptyList, err
	}

	streams := make(map[string]downloader.Stream, len(pornhubs))
	for _, data := range pornhubs {
		realURL := data.VideoURL
		if realURL == "" {
			continue
		}
		size, err := request.Size(realURL, url, config)
		if err != nil {
			return downloader.EmptyList, err
		}
		urlData := downloader.URL{
			URL:  realURL,
			Size: size,
			Ext:  "mp4",
		}
		var quality string
		switch data.Quality.(type) {
		case string:
			quality = data.Quality.(string)
		case []int:
			quality = strconv.Itoa(data.Quality.([]int)[0])
		}
		streams[quality] = downloader.Stream{
			URLs:    []downloader.URL{urlData},
			Size:    size,
			Quality: fmt.Sprintf("%sP", data.Quality),
		}
	}
	// fmt.Println(streams)

	return []downloader.Data{
		{
			Site:    "Pornhub pornhub.com",
			Title:   title,
			Type:    "video",
			Streams: streams,
			URL:     url,
			Length:  duration,
		},
	}, nil
}
