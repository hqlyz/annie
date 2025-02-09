package iqiyi

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/hqlyz/annie/myconfig"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/parser"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

type iqiyi struct {
	Code string `json:"code"`
	Data struct {
		VP struct {
			Du  string `json:"du"`
			Tkl []struct {
				Vs []struct {
					Bid   int    `json:"bid"`
					Scrsz string `json:"scrsz"`
					Vsize int64  `json:"vsize"`
					Fs    []struct {
						L string `json:"l"`
						B int64  `json:"b"`
					} `json:"fs"`
				} `json:"vs"`
			} `json:"tkl"`
		} `json:"vp"`
	} `json:"data"`
	Msg string `json:"msg"`
}

type iqiyiURL struct {
	L string `json:"l"`
}

type videoInfo struct {
	Duration int `json:"duration"`
}

const iqiyiReferer = "https://www.iqiyi.com"

func getMacID() string {
	var macID string
	chars := []string{
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "n", "m", "o", "p", "q", "r", "s", "t", "u", "v",
		"w", "x", "y", "z", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
	}
	size := len(chars)
	for i := 0; i < 32; i++ {
		macID += chars[rand.Intn(size)]
	}
	return macID
}

func getVF(params string) string {
	var suffix string
	for j := 0; j < 8; j++ {
		for k := 0; k < 4; k++ {
			var v8 int
			v4 := 13 * (66*k + 27*j) % 35
			if v4 >= 10 {
				v8 = v4 + 88
			} else {
				v8 = v4 + 49
			}
			suffix += string(v8) // string(97) -> "a"
		}
	}
	params += suffix

	return utils.Md5(params)
}

func getVPS(tvid, vid string, config myconfig.Config) (iqiyi, error) {
	t := time.Now().Unix() * 1000
	host := "http://cache.video.qiyi.com"
	params := fmt.Sprintf(
		"/vps?tvid=%s&vid=%s&v=0&qypid=%s_12&src=01012001010000000000&t=%d&k_tag=1&k_uid=%s&rs=1",
		tvid, vid, tvid, t, getMacID(),
	)
	vf := getVF(params)
	apiURL := fmt.Sprintf("%s%s&vf=%s", host, params, vf)
	info, err := request.Get(apiURL, iqiyiReferer, nil, config)
	if err != nil {
		return iqiyi{}, err
	}
	var data iqiyi
	json.Unmarshal([]byte(info), &data)
	return data, nil
}

// Extract is the main function for extracting data
func Extract(url string, config myconfig.Config) ([]downloader.Data, error) {
	html, err := request.Get(url, iqiyiReferer, nil, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	// ioutil.WriteFile("iqiyi.html", []byte(html), 0644)
	tvid := utils.MatchOneOf(
		url,
		`#curid=(.+)_`,
		`tvid=([^&]+)`,
	)
	if tvid == nil {
		tvid = utils.MatchOneOf(
			html,
			`data-player-tvid="([^"]+)"`,
			`param\['tvid'\]\s*=\s*"(.+?)"`,
			`"tvid":"(\d+)"`,
		)
	}
	vid := utils.MatchOneOf(
		url,
		`#curid=.+_(.*)$`,
		`vid=([^&]+)`,
	)
	if vid == nil {
		vid = utils.MatchOneOf(
			html,
			`data-player-videoid="([^"]+)"`,
			`param\['vid'\]\s*=\s*"(.+?)"`,
			`"vid":"(\w+)"`,
		)
	}
	doc, err := parser.GetDoc(html)
	if err != nil {
		return downloader.EmptyList, err
	}
	title := strings.TrimSpace(doc.Find("h1>a").First().Text())
	var (
		sub       string
		thumbnail string
		duration  int
	)
	thumbnailArr := utils.MatchOneOf(html, `property="og:image" content="(.+?)"`)
	if thumbnailArr != nil {
		thumbnail = thumbnailArr[1]
	} else {
		thumbnail = myconfig.DefaultThumbnail
	}
	videoInfoArr := utils.MatchOneOf(html, `:video-info='(.+?)'`)
	var vinfo videoInfo
	if videoInfoArr != nil {
		err = json.Unmarshal([]byte(videoInfoArr[1]), &vinfo)
		if err != nil {
			return downloader.EmptyList, err
		}
		duration = vinfo.Duration
	} else {
		duration = 0
	}
	for _, k := range []string{"span", "em"} {
		if sub != "" {
			break
		}
		sub = strings.TrimSpace(doc.Find("h1>" + k).First().Text())
	}
	title += sub
	if title == "" {
		title = doc.Find("title").Text()
	}
	videoDatas, err := getVPS(tvid[1], vid[1], config)
	if err != nil {
		return downloader.EmptyList, err
	}
	if videoDatas.Code != "A00000" {
		return downloader.EmptyList, fmt.Errorf("can't play this video: %s", videoDatas.Msg)
	}
	streams := map[string]downloader.Stream{}
	urlPrefix := videoDatas.Data.VP.Du
	for _, video := range videoDatas.Data.VP.Tkl[0].Vs {
		urls := make([]downloader.URL, len(video.Fs))
		for index, v := range video.Fs {
			realURLData, err := request.Get(urlPrefix+v.L, iqiyiReferer, nil, config)
			if err != nil {
				return downloader.EmptyList, err
			}
			var realURL iqiyiURL
			json.Unmarshal([]byte(realURLData), &realURL)
			_, ext, err := utils.GetNameAndExt(realURL.L, config)
			if err != nil {
				return downloader.EmptyList, err
			}
			urls[index] = downloader.URL{
				URL:  realURL.L,
				Size: v.B,
				Ext:  ext,
			}
		}
		streams[strconv.Itoa(video.Bid)] = downloader.Stream{
			URLs:    urls,
			Size:    video.Vsize,
			Quality: video.Scrsz,
		}
	}

	return []downloader.Data{
		{
			Site:      "爱奇艺 iqiyi.com",
			Title:     title,
			Type:      "video",
			Streams:   streams,
			URL:       url,
			Thumbnail: thumbnail,
			Length:    strconv.Itoa(duration),
		},
	}, nil
}
