package universal

import (
	"fmt"

	"github.com/hqlyz/annie/myconfig"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

// Extract is the main function for extracting data
func Extract(url string, config myconfig.Config) ([]downloader.Data, error) {
	fmt.Println()
	fmt.Println("videohandler doesn't support this URL right now, but it will try to download it directly")

	filename, ext, err := utils.GetNameAndExt(url, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	size, err := request.Size(url, url, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	urlData := downloader.URL{
		URL:  url,
		Size: size,
		Ext:  ext,
	}
	streams := map[string]downloader.Stream{
		"default": {
			URLs: []downloader.URL{urlData},
			Size: size,
		},
	}
	contentType, err := request.ContentType(url, url, config)
	if err != nil {
		return downloader.EmptyList, err
	}

	return []downloader.Data{
		{
			Site:    "Universal",
			Title:   filename,
			Type:    contentType,
			Streams: streams,
			URL:     url,
		},
	}, nil
}
