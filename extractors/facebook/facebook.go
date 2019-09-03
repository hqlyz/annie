package facebook

import (
	"fmt"
	"strings"

	"github.com/hqlyz/annie/myconfig"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

// Extract is the main function for extracting data
func Extract(destURL string, config myconfig.Config) ([]downloader.Data, error) {
	var err error
	html, err := request.Get(destURL, destURL, nil, config)
	if err != nil {
		fmt.Println(err)
		return downloader.EmptyList, err
	}
	// ioutil.WriteFile("fb.html", []byte(html), 0644)
	title := utils.MatchOneOf(html, `<title id="pageTitle">(.+)</title>`)[1]

	thumbnail := myconfig.DefaultThumbnail
	thumbnailArr := utils.MatchOneOf(html, `meta\s*property="twitter:image" content="(.+?)"\s*/>`)
	if thumbnailArr != nil {
		thumbnail = strings.Replace(thumbnailArr[1], "&amp;", "&", -1)
	}

	streams := map[string]downloader.Stream{}
	for _, quality := range []string{"sd", "hd"} {
		u := utils.MatchOneOf(
			html, fmt.Sprintf(`%s_src:"(.+?)"`, quality),
		)[1]
		size, err := request.Size(u, destURL, config)
		if err != nil {
			return downloader.EmptyList, err
		}
		urlData := downloader.URL{
			URL:  u,
			Size: size,
			Ext:  "mp4",
		}
		streams[quality] = downloader.Stream{
			URLs:    []downloader.URL{urlData},
			Size:    size,
			Quality: quality,
		}
	}

	return []downloader.Data{
		{
			Site:      "Facebook facebook.com",
			Title:     title,
			Type:      "video",
			Streams:   streams,
			URL:       destURL,
			Thumbnail: thumbnail,
			Length:    "0",
		},
	}, nil
}
