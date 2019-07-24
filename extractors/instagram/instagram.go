package instagram

import (
	"encoding/json"
	"io/ioutil"
	"strconv"

	"github.com/hqlyz/annie/myconfig"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/parser"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

type instagram struct {
	EntryData struct {
		PostPage []struct {
			Graphql struct {
				ShortcodeMedia struct {
					DisplayURL  string `json:"display_url"`
					VideoURL    string `json:"video_url"`
					EdgeSidecar struct {
						Edges []struct {
							Node struct {
								DisplayURL string `json:"display_url"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"edge_sidecar_to_children"`
					Duration  float64 `json:"video_duration"`
					Thumbnail string  `json:"thumbnail_src"`
				} `json:"shortcode_media"`
			} `json:"graphql"`
		} `json:"PostPage"`
	} `json:"entry_data"`
}

// Extract is the main function for extracting data
func Extract(url string, config myconfig.Config) ([]downloader.Data, error) {
	html, err := request.Get(url, url, nil, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	ioutil.WriteFile("E:/instagram.html", []byte(html), 0644)
	// get the title
	doc, err := parser.GetDoc(html)
	if err != nil {
		return downloader.EmptyList, err
	}
	title := parser.Title(doc)

	dataString := utils.MatchOneOf(html, `window\._sharedData\s*=\s*(.*);`)[1]
	var data instagram
	json.Unmarshal([]byte(dataString), &data)

	var realURL, dataType string
	var size int64
	streams := map[string]downloader.Stream{}

	if data.EntryData.PostPage[0].Graphql.ShortcodeMedia.VideoURL != "" {
		// Data
		dataType = "video"
		realURL = data.EntryData.PostPage[0].Graphql.ShortcodeMedia.VideoURL
		size, err = request.Size(realURL, url, config)
		if err != nil {
			return downloader.EmptyList, err
		}
		streams["default"] = downloader.Stream{
			URLs: []downloader.URL{
				{
					URL:  realURL,
					Size: size,
					Ext:  "mp4",
				},
			},
			Size: size,
		}
	} else {
		// Image
		dataType = "image"
		if data.EntryData.PostPage[0].Graphql.ShortcodeMedia.EdgeSidecar.Edges == nil {
			// Single
			realURL = data.EntryData.PostPage[0].Graphql.ShortcodeMedia.DisplayURL
			size, err = request.Size(realURL, url, config)
			if err != nil {
				return downloader.EmptyList, err
			}
			streams["default"] = downloader.Stream{
				URLs: []downloader.URL{
					{
						URL:  realURL,
						Size: size,
						Ext:  "jpg",
					},
				},
				Size: size,
			}
		} else {
			// Album
			var totalSize int64
			var urls []downloader.URL
			for _, u := range data.EntryData.PostPage[0].Graphql.ShortcodeMedia.EdgeSidecar.Edges {
				realURL = u.Node.DisplayURL
				size, err = request.Size(realURL, url, config)
				if err != nil {
					return downloader.EmptyList, err
				}
				urlData := downloader.URL{
					URL:  realURL,
					Size: size,
					Ext:  "jpg",
				}
				urls = append(urls, urlData)
				totalSize += size
			}
			streams["default"] = downloader.Stream{
				URLs: urls,
				Size: totalSize,
			}
		}
	}
	var (
		dur       int
		thumbnail string
	)
	if dataType == "video" {
		dur = int(data.EntryData.PostPage[0].Graphql.ShortcodeMedia.Duration)
		thumbnail = data.EntryData.PostPage[0].Graphql.ShortcodeMedia.Thumbnail
	} else {
		dur = 0
		thumbnail = ""
	}
	return []downloader.Data{
		{
			Site:      "Instagram instagram.com",
			Title:     title,
			Type:      dataType,
			Streams:   streams,
			URL:       url,
			Length:    strconv.Itoa(dur),
			Thumbnail: thumbnail,
		},
	}, nil
}
