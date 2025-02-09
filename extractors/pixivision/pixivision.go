package pixivision

import (
	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/myconfig"
	"github.com/hqlyz/annie/parser"
	"github.com/hqlyz/annie/request"
)

// Extract is the main function for extracting data
func Extract(url string, config myconfig.Config) ([]downloader.Data, error) {
	html, err := request.Get(url, url, nil, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	title, urls, err := parser.GetImages(url, html, "am__work__illust  ", nil, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	streams := map[string]downloader.Stream{
		"default": {
			URLs: urls,
			Size: 0,
		},
	}

	return []downloader.Data{
		{
			Site:    "pixivision pixivision.net",
			Title:   title,
			Type:    "image",
			Streams: streams,
			URL:     url,
		},
	}, nil
}
