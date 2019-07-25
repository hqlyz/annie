package bilibili

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/myconfig"
	"github.com/hqlyz/annie/parser"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

const (
	bilibiliAPI        = "https://interface.bilibili.com/v2/playurl?"
	bilibiliBangumiAPI = "https://bangumi.bilibili.com/player/web_api/v2/playurl?"
	bilibiliTokenAPI   = "https://api.bilibili.com/x/player/playurl/token?"
)

const (
	// BiliBili blocks keys from time to time.
	// You can extract from the Android client or bilibiliPlayer.min.js
	appKey = "iVGUTjsxvpLeuDCf"
	secKey = "aHRmhWMLkdeMuILqORnYZocwMBpMEOdt"
)

const referer = "https://www.bilibili.com"

var utoken string

func genAPI(aid, cid string, bangumi bool, quality string, seasonType string, config myconfig.Config) (string, error) {
	var (
		err        error
		baseAPIURL string
		params     string
	)
	if config.Cookie != "" && utoken == "" {
		utoken, err = request.Get(
			fmt.Sprintf("%said=%s&cid=%s", bilibiliTokenAPI, aid, cid),
			referer,
			nil,
			config,
		)
		if err != nil {
			return "", err
		}
		var t token
		err = json.Unmarshal([]byte(utoken), &t)
		if err != nil {
			return "", err
		}
		if t.Code != 0 {
			return "", fmt.Errorf("cookie error: %s", t.Message)
		}
		utoken = t.Data.Token
	}
	if bangumi {
		// The parameters need to be sorted by name
		// qn=0 flag makes the CDN address different every time
		// quality=116(1080P 60) is the highest quality so far
		params = fmt.Sprintf(
			"appkey=%s&cid=%s&module=bangumi&otype=json&qn=%s&quality=%s&season_type=%s&type=",
			appKey, cid, quality, quality, seasonType,
		)
		baseAPIURL = bilibiliBangumiAPI
	} else {
		params = fmt.Sprintf(
			"appkey=%s&cid=%s&otype=json&qn=%s&quality=%s&type=",
			appKey, cid, quality, quality,
		)
		baseAPIURL = bilibiliAPI
	}
	// bangumi utoken also need to put in params to sign, but the ordinary video doesn't need
	api := fmt.Sprintf(
		"%s%s&sign=%s", baseAPIURL, params, utils.Md5(params+secKey),
	)
	if !bangumi && utoken != "" {
		api = fmt.Sprintf("%s&utoken=%s", api, utoken)
	}
	return api, nil
}

func genURL(durl []dURLData) ([]downloader.URL, int64) {
	var size int64
	urls := make([]downloader.URL, len(durl))
	for index, data := range durl {
		size += data.Size
		urls[index] = downloader.URL{
			URL:  data.URL,
			Size: data.Size,
			Ext:  "flv",
		}
	}
	return urls, size
}

type bilibiliOptions struct {
	url       string
	html      string
	bangumi   bool
	aid       string
	cid       string
	page      int
	subtitle  string
	thumbnail string
	length    string
}

type playInfo struct {
	Data    dataInfo `json:"data"`
	Session string   `json:"session"`
	Ttl     int      `json:"ttl"`
}

type dataInfo struct {
	From       string `json:"from"`
	Result     string `json:"result"`
	TimeLength int    `json:"timelength"`
}

func extractBangumi(url, html string, config myconfig.Config) ([]downloader.Data, error) {
	dataString := utils.MatchOneOf(html, `window.__INITIAL_STATE__=(.+?);\(function`)[1]
	var (
		data      bangumiData
		thumbnail string
		length    string
	)
	err := json.Unmarshal([]byte(dataString), &data)
	if err != nil {
		return downloader.EmptyList, err
	}
	thumbnailString := utils.MatchOneOf(html, `property="og:image" content="(.+?)"`)
	if thumbnailString != nil {
		thumbnail = thumbnailString[1]
	} else {
		thumbnail = ""
	}

	playInfoString := utils.MatchOneOf(html, `__playinfo__=(.+?)</script><script>`)
	if playInfoString != nil {
		var pInfo playInfo
		err := json.Unmarshal([]byte(playInfoString[1]), &pInfo)
		if err != nil {
			fmt.Println(err.Error())
			length = "0"
		} else {
			length = strconv.Itoa(pInfo.Data.TimeLength / 1000)
		}
	} else {
		length = "0"
	}
	fmt.Println(thumbnail)
	fmt.Println(length)
	if !config.Playlist {
		options := bilibiliOptions{
			url:       url,
			html:      html,
			bangumi:   true,
			aid:       strconv.Itoa(data.EpInfo.Aid),
			cid:       strconv.Itoa(data.EpInfo.Cid),
			thumbnail: thumbnail,
			length:    length,
		}
		return []downloader.Data{bilibiliDownload(options, config)}, nil
	}

	// handle bangumi playlist
	needDownloadItems := utils.NeedDownloadList(len(data.EpList), config)
	extractedData := make([]downloader.Data, len(needDownloadItems))
	wgp := utils.NewWaitGroupPool(config.ThreadNumber)
	dataIndex := 0
	for index, u := range data.EpList {
		if !utils.ItemInSlice(index+1, needDownloadItems) {
			continue
		}
		wgp.Add()
		id := u.EpID
		if id == 0 {
			id = u.ID
		}
		// html content can't be reused here
		options := bilibiliOptions{
			url:       fmt.Sprintf("https://www.bilibili.com/bangumi/play/ep%d", id),
			bangumi:   true,
			aid:       strconv.Itoa(u.Aid),
			cid:       strconv.Itoa(u.Cid),
			thumbnail: thumbnail,
			length:    length,
		}
		go func(index int, options bilibiliOptions, extractedData []downloader.Data) {
			defer wgp.Done()
			extractedData[index] = bilibiliDownload(options, config)
		}(dataIndex, options, extractedData)
		dataIndex++
	}
	wgp.Wait()
	return extractedData, nil
}

func getMultiPageData(html string) (*multiPage, error) {
	var data multiPage
	multiPageDataString := utils.MatchOneOf(
		html, `window.__INITIAL_STATE__=(.+?);\(function`,
	)
	if multiPageDataString == nil {
		return &data, errors.New("this page has no playlist")
	}
	err := json.Unmarshal([]byte(multiPageDataString[1]), &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func extractNormalVideo(url, html string, config myconfig.Config) ([]downloader.Data, error) {
	pageData, err := getMultiPageData(html)
	if err != nil {
		return downloader.EmptyList, err
	}
	var (
		thumbnail string
		length    string
	)

	thumbnailString := utils.MatchOneOf(html, `property="og:image" content="(.+?)"`)
	if thumbnailString != nil {
		thumbnail = thumbnailString[1]
	} else {
		thumbnail = ""
	}

	playInfoString := utils.MatchOneOf(html, `__playinfo__=(.+?)</script><script>`)
	if playInfoString != nil {
		var pInfo playInfo
		err := json.Unmarshal([]byte(playInfoString[1]), &pInfo)
		if err != nil {
			fmt.Println(err.Error())
			length = "0"
		} else {
			length = strconv.Itoa(pInfo.Data.TimeLength / 1000)
		}
	} else {
		length = "0"
	}
	if !config.Playlist {
		// handle URL that has a playlist, mainly for unified titles
		// <h1> tag does not include subtitles
		// bangumi doesn't need this
		pageString := utils.MatchOneOf(url, `\?p=(\d+)`)
		var (
			p int
		)
		if pageString == nil {
			// https://www.bilibili.com/video/av20827366/
			p = 1
		} else {
			// https://www.bilibili.com/video/av20827366/?p=2
			p, _ = strconv.Atoi(pageString[1])
		}

		page := pageData.VideoData.Pages[p-1]
		options := bilibiliOptions{
			url:       url,
			html:      html,
			aid:       pageData.Aid,
			cid:       strconv.Itoa(page.Cid),
			page:      p,
			thumbnail: thumbnail,
			length:    length,
		}
		// "part":"" or "part":"Untitled"
		if page.Part == "Untitled" || len(pageData.VideoData.Pages) == 1 {
			options.subtitle = ""
		} else {
			options.subtitle = page.Part
		}
		return []downloader.Data{bilibiliDownload(options, config)}, nil
	}

	// handle normal video playlist
	// https://www.bilibili.com/video/av20827366/?p=1
	needDownloadItems := utils.NeedDownloadList(len(pageData.VideoData.Pages), config)
	extractedData := make([]downloader.Data, len(needDownloadItems))
	wgp := utils.NewWaitGroupPool(config.ThreadNumber)
	dataIndex := 0
	for index, u := range pageData.VideoData.Pages {
		if !utils.ItemInSlice(index+1, needDownloadItems) {
			continue
		}
		wgp.Add()
		options := bilibiliOptions{
			url:       url,
			html:      html,
			aid:       pageData.Aid,
			cid:       strconv.Itoa(u.Cid),
			subtitle:  u.Part,
			page:      u.Page,
			thumbnail: thumbnail,
			length:    length,
		}
		go func(index int, options bilibiliOptions, extractedData []downloader.Data) {
			defer wgp.Done()
			extractedData[index] = bilibiliDownload(options, config)
		}(dataIndex, options, extractedData)
		dataIndex++
	}
	wgp.Wait()
	return extractedData, nil
}

// Extract is the main function for extracting data
func Extract(url string, config myconfig.Config) ([]downloader.Data, error) {
	var err error
	html, err := request.Get(url, referer, nil, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	// ioutil.WriteFile("bili.html", []byte(html), 0666)
	if strings.Contains(url, "bangumi") {
		// handle bangumi
		return extractBangumi(url, html, config)
	}
	// handle normal video
	return extractNormalVideo(url, html, config)
}

// bilibiliDownload is the download function for a single URL
func bilibiliDownload(options bilibiliOptions, config myconfig.Config) downloader.Data {
	var (
		err        error
		html       string
		seasonType string
	)
	if options.html != "" {
		// reuse html string, but this can't be reused in case of playlist
		html = options.html
	} else {
		html, err = request.Get(options.url, referer, nil, config)
		if err != nil {
			return downloader.EmptyData(options.url, err)
		}
	}
	if options.bangumi {
		seasonType = utils.MatchOneOf(html, `"season_type":(\d+)`, `"ssType":(\d+)`)[1]
	}

	// Get "accept_quality" and "accept_description"
	// "accept_description":["高清 1080P","高清 720P","清晰 480P","流畅 360P"],
	// "accept_quality":[80,48,32,16],
	api, err := genAPI(options.aid, options.cid, options.bangumi, "15", seasonType, config)
	if err != nil {
		return downloader.EmptyData(options.url, err)
	}
	jsonString, err := request.Get(api, referer, nil, config)
	if err != nil {
		return downloader.EmptyData(options.url, err)
	}
	var quality qualityInfo
	err = json.Unmarshal([]byte(jsonString), &quality)
	if err != nil {
		return downloader.EmptyData(options.url, err)
	}

	streams := make(map[string]downloader.Stream, len(quality.Quality))
	for _, q := range quality.Quality {
		apiURL, err := genAPI(options.aid, options.cid, options.bangumi, strconv.Itoa(q), seasonType, config)
		if err != nil {
			return downloader.EmptyData(options.url, err)
		}
		jsonString, err := request.Get(apiURL, referer, nil, config)
		if err != nil {
			return downloader.EmptyData(options.url, err)
		}
		var data bilibiliData
		err = json.Unmarshal([]byte(jsonString), &data)
		if err != nil {
			return downloader.EmptyData(options.url, err)
		}

		// Avoid duplicate streams
		if _, ok := streams[strconv.Itoa(data.Quality)]; ok {
			continue
		}

		urls, size := genURL(data.DURL)
		streams[strconv.Itoa(data.Quality)] = downloader.Stream{
			URLs:    urls,
			Size:    size,
			Quality: qualityString[data.Quality],
		}
	}

	// get the title
	doc, err := parser.GetDoc(html)
	if err != nil {
		return downloader.EmptyData(options.url, err)
	}
	title := parser.Title(doc)
	if options.subtitle != "" {
		tempTitle := fmt.Sprintf("%s %s", title, options.subtitle)
		if len([]rune(tempTitle)) > utils.MAXLENGTH {
			tempTitle = fmt.Sprintf("%s P%d %s", title, options.page, options.subtitle)
		}
		title = tempTitle
	}

	downloader.Caption(
		fmt.Sprintf("https://comment.bilibili.com/%s.xml", options.cid),
		options.url, title, "xml",
		config,
	)

	return downloader.Data{
		Site:      "哔哩哔哩 bilibili.com",
		Title:     title,
		Type:      "video",
		Streams:   streams,
		URL:       options.url,
		Thumbnail: options.thumbnail,
		Length:    options.length,
	}
}
