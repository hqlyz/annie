package douyin

import (
	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/myconfig"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

// Extract is the main function for extracting data
func Extract(url string, config myconfig.Config) ([]downloader.Data, error) {
	var err error
	html, err := request.Get(url, url, nil, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	// ioutil.WriteFile("douyin.html", []byte(html), 0644)
	var (
		title     string
		thumbnail string
	)
	desc := utils.MatchOneOf(html, `<p class="desc">(.+?)</p>`)
	if desc != nil {
		title = desc[1]
	} else {
		title = "抖音短视频"
	}
	thumbnailArr := utils.MatchOneOf(html, `cover: "(.+?)"`)
	if thumbnailArr != nil {
		thumbnail = thumbnailArr[1]
	} else {
		thumbnail = myconfig.DefaultThumbnail
	}
	realURL := utils.MatchOneOf(html, `playAddr: "(.+?)"`)[1]
	size, err := request.Size(realURL, url, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	urlData := downloader.URL{
		URL:  realURL,
		Size: size,
		Ext:  "mp4",
	}
	streams := map[string]downloader.Stream{
		"default": {
			URLs:    []downloader.URL{urlData},
			Size:    size,
			Quality: "Clear",
		},
	}
	return []downloader.Data{
		{
			Site:      "抖音 douyin.com",
			Title:     title,
			Type:      "video",
			Streams:   streams,
			URL:       url,
			Thumbnail: thumbnail,
			Length:    "0",
		},
	}, nil
}
