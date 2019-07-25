package bcy

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/hqlyz/annie/myconfig"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/parser"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

type bcyData struct {
	Detail struct {
		PostData struct {
			Multi []struct {
				OriginalPath string `json:"original_path"`
			} `json:"multi"`
		} `json:"post_data"`
	} `json:"detail"`
}

// 目前失效，暂不处理
// Extract is the main function for extracting data
func Extract(url string, config myconfig.Config) ([]downloader.Data, error) {
	var err error
	html, err := request.Get(url, url, nil, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	ioutil.WriteFile("bcy.html", []byte(html), 0644)
	// parse json data
	rep := strings.NewReplacer(`\"`, `"`, `\\`, `\`)
	jsonString := rep.Replace(utils.MatchOneOf(html, `JSON.parse\("(.+?)"\);`)[1])
	var data bcyData
	json.Unmarshal([]byte(jsonString), &data)

	doc, err := parser.GetDoc(html)
	if err != nil {
		return downloader.EmptyList, err
	}
	title := strings.Replace(parser.Title(doc), " - 半次元 banciyuan - ACG爱好者社区", "", -1)

	urls := make([]downloader.URL, 0, len(data.Detail.PostData.Multi))
	var totalSize int64
	for _, img := range data.Detail.PostData.Multi {
		size, err := request.Size(img.OriginalPath, url, config)
		if err != nil {
			return downloader.EmptyList, err
		}
		totalSize += size
		_, ext, err := utils.GetNameAndExt(img.OriginalPath, config)
		if err != nil {
			return downloader.EmptyList, err
		}
		urls = append(urls, downloader.URL{
			URL:  img.OriginalPath,
			Size: size,
			Ext:  ext,
		})
	}
	streams := map[string]downloader.Stream{
		"default": {
			URLs: urls,
			Size: totalSize,
		},
	}
	return []downloader.Data{
		{
			Site:    "半次元 bcy.net",
			Title:   title,
			Type:    "image",
			Streams: streams,
			URL:     url,
		},
	}, nil
}
