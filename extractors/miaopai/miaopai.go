package miaopai

import (
	"encoding/json"
	"fmt"

	"github.com/hqlyz/annie/myconfig"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

type miaopai struct {
	Data struct {
		Description string `json:"description"`
		MetaData    []struct {
			URLs struct {
				M string `json:"m"`
			} `json:"play_urls"`
		} `json:"meta_data"`
	} `json:"data"`
}

// Extract is the main function for extracting data
func Extract(url string, config myconfig.Config) ([]downloader.Data, error) {
	id := utils.MatchOneOf(url, `/media/([^\./]+)`, `/show(?:/channel)?/([^\./]+)`)[1]
	jsonString, err := request.Get(
		fmt.Sprintf("https://n.miaopai.com/api/aj_media/info.json?smid=%s", id), url, nil, config,
	)
	if err != nil {
		return downloader.EmptyList, err
	}
	var data miaopai
	json.Unmarshal([]byte(jsonString), &data)

	realURL := data.Data.MetaData[0].URLs.M
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
			URLs: []downloader.URL{urlData},
			Size: size,
		},
	}

	return []downloader.Data{
		{
			Site:    "秒拍 miaopai.com",
			Title:   data.Data.Description,
			Type:    "video",
			Streams: streams,
			URL:     url,
		},
	}, nil
}
