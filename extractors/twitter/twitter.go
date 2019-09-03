package twitter

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hqlyz/annie/myconfig"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

type twitter struct {
	Track struct {
		URL string `json:"playbackUrl"`
	} `json:"track"`
	TweetID  string
	Username string
}

// Extract is the main function for extracting data
func Extract(uri string, config myconfig.Config) ([]downloader.Data, error) {
	html, err := request.Get(uri, uri, nil, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	// ioutil.WriteFile("twitter.html", []byte(html), 0644)
	usernameArr := utils.MatchOneOf(html, `property="og:title"\s+content="(.+)"`)
	if len(usernameArr) < 2 {
		return downloader.EmptyList, err
	}
	username := usernameArr[1]

	thumbnail := myconfig.DefaultThumbnail
	thumb := utils.MatchOneOf(html, `meta\s*property="og:image" content="(.+?)">`)
	if thumb != nil {
		thumbnail = thumb[1]
	}

	tweetIDArr := utils.MatchOneOf(uri, `(status|statuses)/(\d+)`)
	if len(tweetIDArr) < 3 {
		return downloader.EmptyList, err
	}
	tweetID := utils.MatchOneOf(uri, `(status|statuses)/(\d+)`)[2]
	api := fmt.Sprintf(
		"https://api.twitter.com/1.1/videos/tweet/config/%s.json", tweetID,
	)
	headers := map[string]string{
		"Authorization": "Bearer AAAAAAAAAAAAAAAAAAAAAIK1zgAAAAAA2tUWuhGZ2JceoId5GwYWU5GspY4%3DUq7gzFoCZs1QfwGoVdvSac3IniczZEYXIcDyumCauIXpcAPorE",
	}
	jsonString, err := request.Get(api, uri, headers, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	// ioutil.WriteFile("twitter.json", []byte(jsonString), 0644)
	var twitterData twitter
	json.Unmarshal([]byte(jsonString), &twitterData)
	twitterData.TweetID = tweetID
	twitterData.Username = username
	extractedData, err := download(twitterData, uri, config, thumbnail)
	if err != nil {
		return downloader.EmptyList, err
	}
	return extractedData, nil
}

func download(data twitter, uri string, config myconfig.Config, thumbnail string) ([]downloader.Data, error) {
	var (
		err  error
		size int64
	)
	streams := make(map[string]downloader.Stream)
	switch {
	// if video file is m3u8 and ts
	case strings.Contains(data.Track.URL, ".m3u8"):
		m3u8urls, err := utils.M3u8URLs(data.Track.URL, config)
		if err != nil {
			return downloader.EmptyList, err
		}
		for index, m3u8 := range m3u8urls {
			var totalSize int64
			var urls []downloader.URL
			ts, err := utils.M3u8URLs(m3u8, config)
			if err != nil {
				return downloader.EmptyList, err
			}
			for _, i := range ts {
				size, err := request.Size(i, uri, config)
				if err != nil {
					return downloader.EmptyList, err
				}
				temp := downloader.URL{
					URL:  i,
					Size: size,
					Ext:  "ts",
				}
				totalSize += size
				urls = append(urls, temp)
			}
			qualityStringArr := utils.MatchOneOf(m3u8, `/(\d+x\d+)/`)
			if len(qualityStringArr) < 2 {
				return downloader.EmptyList, err
			}
			qualityString := qualityStringArr[1]
			quality := strconv.Itoa(index + 1)
			streams[quality] = downloader.Stream{
				Quality: qualityString,
				URLs:    urls,
				Size:    totalSize,
			}
		}

	// if video file is mp4
	case strings.Contains(data.Track.URL, ".mp4"):
		size, err = request.Size(data.Track.URL, uri, config)
		if err != nil {
			return downloader.EmptyList, err
		}
		urlData := downloader.URL{
			URL:  data.Track.URL,
			Size: size,
			Ext:  "mp4",
		}
		streams["default"] = downloader.Stream{
			URLs: []downloader.URL{urlData},
			Size: size,
		}
	}

	return []downloader.Data{
		{
			Site:      "Twitter twitter.com",
			Title:     fmt.Sprintf("%s %s", data.Username, data.TweetID),
			Type:      "video",
			Streams:   streams,
			URL:       uri,
			Thumbnail: thumbnail,
			Length:    "0",
		},
	}, nil
}
