package youtube

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/myconfig"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
)

type args struct {
	Title  string `json:"title"`
	Stream string `json:"adaptive_fmts"`
	// not every page has `adaptive_fmts` field https://youtu.be/DNaOZovrSVo
	Stream2        string `json:"url_encoded_fmt_stream_map"`
	PlayerResponse string `json:"player_response"`
}

type assets struct {
	JS string `json:"js"`
}

type thumbnail struct {
	Thumbnails []thumbnailsInfo `json:"thumbnails"`
}

type thumbnailsInfo struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type youtubeData struct {
	Args   args   `json:"args"`
	Assets assets `json:"assets"`
}

type videoDetails struct {
	Thumbnail     thumbnail `json:"thumbnail"`
	LengthSeconds string    `json:"lengthSeconds"`
	Title         string    `json:"title"`
}

type playerResponseData struct {
	VideoDetails videoDetails `json:"videoDetails"`
	Captions     captions     `json:"captions"`
}

type captions struct {
	PlayerCaptionsTracklistRenderer playerCaptionsTracklistRenderer `json:"playerCaptionsTracklistRenderer"`
}

type playerCaptionsTracklistRenderer struct {
	CaptionTracks []captionTracksItem `json:"captionTracks"`
}

type captionTracksItem struct {
	BaseURL string `json:"baseUrl"`
}

// another parse method type
type anotherPlayerResponseData struct {
	VideoDetails videoDetails `json:"videoDetails"`
	Captions     captions     `json:"captions"`
	StreamData   streamData   `json:"streamingData"`
}

type streamData struct {
	AdaptiveFormats []adaptiveFormatsItem `json:"adaptiveFormats"`
}

type adaptiveFormatsItem struct {
	Itag          int    `json:"itag"`
	URL           string `json:"url"`
	MimeType      string `json:"mimeType"`
	ContentLength string `json:"contentLength"`
	Quality       string `json:"qualityLabel"`
	Cipher        string `json:"cipher"`
}

const referer = "https://www.youtube.com"
const generalBaseJSURL = "https://www.youtube.com/yts/jsbin/player_ias-vfle9vlRm/en_US/base.js"
const generalBaseJSURLKey = "ytgbjsurlkey"

// var tokensCache = make(map[string][]string)

// func getSig(sig, js string) (string, error) {
// 	sigURL := fmt.Sprintf("https://www.youtube.com%s", js)
// 	tokens, ok := tokensCache[sigURL]
// 	if !ok {
// 		html, err := request.Get(sigURL, referer, nil)
// 		if err != nil {
// 			return "", err
// 		}
// 		tokens, err = getSigTokens(html)
// 		if err != nil {
// 			return "", err
// 		}
// 		tokensCache[sigURL] = tokens
// 	}
// 	return decipherTokens(tokens, sig), nil
// }

// func genSignedURL(streamURL string, stream url.Values, js string) (string, error) {
// 	var (
// 		realURL, sig string
// 		err          error
// 	)
// 	// fmt.Println(streamURL)
// 	if strings.Contains(streamURL, "signature=") {
// 		// URL itself already has a signature parameter
// 		realURL = streamURL
// 	} else {
// 		// URL has no signature parameter
// 		sig = stream.Get("sig")
// 		if sig == "" {
// 			// Signature need decrypt
// 			sig, err = getSig(stream.Get("s"), js)
// 			if err != nil {
// 				return "", err
// 			}
// 		}
// 		realURL = fmt.Sprintf("%s&signature=%s", streamURL, sig)
// 	}
// 	if !strings.Contains(realURL, "ratebypass") {
// 		realURL += "&ratebypass=yes"
// 	}
// 	return realURL, nil
// }

// Extract is the main function for extracting data
func Extract(uri string, cacheJL *cache.Cache, config myconfig.Config) ([]downloader.Data, error) {
	var err error
	if !config.Playlist {
		return []downloader.Data{youtubeDownload(uri, cacheJL, config)}, nil
	}
	listID := utils.MatchOneOf(uri, `(list|p)=([^/&]+)`)[2]
	if listID == "" {
		return downloader.EmptyList, errors.New("can't get list ID from URL")
	}
	html, err := request.Get("https://www.youtube.com/playlist?list="+listID, referer, nil, config)
	if err != nil {
		return downloader.EmptyList, err
	}
	// "videoId":"OQxX8zgyzuM","thumbnail"
	videoIDs := utils.MatchAll(html, `"videoId":"([^,]+?)","thumbnail"`)
	needDownloadItems := utils.NeedDownloadList(len(videoIDs), config)
	extractedData := make([]downloader.Data, len(needDownloadItems))
	wgp := utils.NewWaitGroupPool(config.ThreadNumber)
	dataIndex := 0
	for index, videoID := range videoIDs {
		if !utils.ItemInSlice(index+1, needDownloadItems) {
			continue
		}
		u := fmt.Sprintf(
			"https://www.youtube.com/watch?v=%s&list=%s", videoID[1], listID,
		)
		wgp.Add()
		go func(index int, u string, extractedData []downloader.Data) {
			defer wgp.Done()
			extractedData[index] = youtubeDownload(u, cacheJL, config)
		}(dataIndex, u, extractedData)
		dataIndex++
	}
	wgp.Wait()
	return extractedData, nil
}

// youtubeDownload download function for single url
func youtubeDownload(uri string, cacheJL *cache.Cache, config myconfig.Config) downloader.Data {
	var err error
	vid := utils.MatchOneOf(
		uri,
		`watch\?v=([^/&]+)`,
		`youtu\.be/([^?/]+)`,
		`embed/([^/?]+)`,
		`v/([^/?]+)`,
	)
	if vid == nil {
		return downloader.EmptyData(uri, errors.New("can't find vid"))
	}
	videoURL := fmt.Sprintf(
		"https://www.youtube.com/watch?v=%s&gl=US&hl=en&has_verified=1&bpctr=9999999999",
		vid[1],
	)
	html, err := request.Get(videoURL, referer, nil, config)
	// ioutil.WriteFile("youtube.html", []byte(html), 0666)
	if err != nil {
		return downloader.EmptyData(uri, err)
	}
	if strings.Contains(html, "Licensed to YouTube by") && !config.SupervisorMode {
		return downloader.EmptyData(uri, errors.New("Can't download copyrighted video"))
	}
	ytplayerArr := utils.MatchOneOf(html, `;ytplayer\.config\s*=\s*({.+?});`)
	if len(ytplayerArr) == 0 {
		// if general method failed, try another method
		anotherData, err := anotherParseMethod(vid[1], referer, config, cacheJL, uri)
		if err != nil {
			return downloader.EmptyData(uri, errors.New("the video is not available"))
		}
		return anotherData
	}
	ytplayer := utils.MatchOneOf(html, `;ytplayer\.config\s*=\s*({.+?});`)[1]
	var youtube youtubeData
	err = json.Unmarshal([]byte(ytplayer), &youtube)
	if err != nil {
		return downloader.EmptyData(uri, err)
	}
	cacheJL.Set(generalBaseJSURLKey, youtube.Assets.JS, time.Hour*24)
	var playerResponseData playerResponseData
	err = json.Unmarshal([]byte(youtube.Args.PlayerResponse), &playerResponseData)
	if err != nil {
		return downloader.EmptyData(uri, err)
	}
	title := playerResponseData.VideoDetails.Title
	streams, err := extractVideoURLS(youtube, uri, cacheJL, config)
	if err != nil {
		return downloader.EmptyData(uri, err)
	}
	captionURL := ""
	if len(playerResponseData.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks) > 0 {
		captionURL = playerResponseData.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks[0].BaseURL
	}

	return downloader.Data{
		Site:       "YouTube youtube.com",
		Title:      title,
		Type:       "video",
		Streams:    streams,
		URL:        uri,
		Thumbnail:  playerResponseData.VideoDetails.Thumbnail.Thumbnails[1].URL,
		Length:     playerResponseData.VideoDetails.LengthSeconds,
		CaptionURL: captionURL,
	}
}

func extractVideoURLS(data youtubeData, referer string, cacheJL *cache.Cache, config myconfig.Config) (map[string]downloader.Stream, error) {
	var youtubeStreams []string
	if config.YouTubeStream2 || data.Args.Stream == "" {
		youtubeStreams = strings.Split(data.Args.Stream2, ",")
	} else {
		youtubeStreams = strings.Split(data.Args.Stream, ",")
	}
	var ext string
	var audio downloader.URL
	var audioWebm downloader.URL
	var audioFound bool
	var audioWebmFound bool
	streams := map[string]downloader.Stream{}

	for _, s := range youtubeStreams {
		// fmt.Println(s)
		stream, err := url.ParseQuery(s)
		if err != nil {
			return nil, err
		}
		itag := stream.Get("itag")
		streamType := stream.Get("type")
		isAudio := strings.HasPrefix(streamType, "audio")

		quality := stream.Get("quality_label")
		if quality == "" {
			quality = stream.Get("quality") // for url_encoded_fmt_stream_map
		}
		if quality != "" {
			quality = fmt.Sprintf("%s %s", quality, streamType)
		} else {
			quality = streamType
		}
		ext = utils.MatchOneOf(streamType, `(\w+)/(\w+);`)[2]
		realURL, err := getDownloadURL(stream, data.Assets.JS, cacheJL, config)
		if err != nil {
			return nil, err
		}
		sizeStr := stream.Get("clen")
		size := int64(0)
		if sizeStr != "" {
			size, err = strconv.ParseInt(sizeStr, 10, 64)
			if err != nil {
				size = int64(0)
			}
		}
		// if err != nil {
		// 	// some stream of the video will return a 404 error,
		// 	// I don't know if it is a problem with the signature algorithm.
		// 	// https://github.com/hqlyz/annie/issues/322
		// 	continue
		// }
		urlData := downloader.URL{
			URL:  realURL,
			Size: size,
			Ext:  ext,
		}
		if isAudio {
			// Audio data for merging with video
			if strings.Contains(quality, "webm") && !audioWebmFound {
				audioWebm = urlData
				audioWebmFound = true
			} else if !strings.Contains(quality, "webm") && !audioFound {
				audio = urlData
				audioFound = true
			}
		}
		streams[itag] = downloader.Stream{
			URLs:    []downloader.URL{urlData},
			Size:    size,
			Quality: quality,
		}
	}

	// `url_encoded_fmt_stream_map`
	if data.Args.Stream == "" {
		return streams, nil
	}

	// Unlike `url_encoded_fmt_stream_map`, all videos in `adaptive_fmts` have no sound,
	// we need download video and audio both and then merge them.
	// Another problem is that even if we add `ratebypass=yes`, the download speed still slow sometimes. https://github.com/hqlyz/annie/issues/191#issuecomment-405449649

	// All videos here have no sound and need to be added separately
	for itag, f := range streams {
		if strings.Contains(f.Quality, "video/") {
			if f.URLs[0].Ext == "webm" {
				f.Size += audioWebm.Size
				f.URLs = append(f.URLs, audioWebm)
			} else {
				f.Size += audio.Size
				f.URLs = append(f.URLs, audio)
			}
			streams[itag] = f
		}
	}
	return streams, nil
}

func anotherParseMethod(vid string, referer string, config myconfig.Config, cacheJL *cache.Cache, uri string) (downloader.Data, error) {
	fmt.Println("try another method to parse youtube video")
	destURL := fmt.Sprintf("https://www.youtube.com/get_video_info?video_id=%s&eurl=https%%3A%%2F%%2Fy", vid)
	html2, err := request.Get(destURL, referer, nil, config)
	if err != nil {
		return downloader.Data{}, err
	}
	// ioutil.WriteFile("youtube2.html", []byte(html2), 0644)
	videoInfo, err := url.ParseQuery(html2)
	if err != nil {
		return downloader.Data{}, err
	}
	if videoInfo.Get("status") == "ok" {
		playerResponseStr, err := url.QueryUnescape(videoInfo.Get("player_response"))
		if err != nil {
			return downloader.Data{}, err
		}
		// ioutil.WriteFile("player_response.json", []byte(playerResponseStr), 0644)
		var anotherPlayerResponse anotherPlayerResponseData
		err = json.Unmarshal([]byte(playerResponseStr), &anotherPlayerResponse)
		if err != nil {
			fmt.Println(err.Error())
			return downloader.Data{}, err
		}

		// if anotherPlayerResponse.StreamData.AdaptiveFormats[0].URL != "" {
		streams := make(map[string]downloader.Stream)
		var ext string
		var audio downloader.URL
		var audioWebm downloader.URL
		var audioFound bool
		var audioWebmFound bool

		for _, s := range anotherPlayerResponse.StreamData.AdaptiveFormats {
			itag := strconv.Itoa(s.Itag)
			streamType := s.MimeType
			isAudio := strings.HasPrefix(streamType, "audio")

			quality := s.Quality
			if quality != "" {
				quality = fmt.Sprintf("%s %s", quality, streamType)
			} else {
				quality = streamType
			}
			ext = utils.MatchOneOf(streamType, `(\w+)/(\w+);`)[2]
			realURL := s.URL
			sizeStr := s.ContentLength
			size := int64(0)
			if sizeStr != "" {
				size, err = strconv.ParseInt(sizeStr, 10, 64)
				if err != nil {
					size = int64(0)
				}
			}
			urlData := downloader.URL{
				URL:  realURL,
				Size: size,
				Ext:  ext,
			}
			if isAudio {
				// Audio data for merging with video
				if strings.Contains(quality, "webm") && !audioWebmFound {
					audioWebm = urlData
					audioWebmFound = true
				} else if !strings.Contains(quality, "webm") && !audioFound {
					audio = urlData
					audioFound = true
				}
			}
			streams[itag] = downloader.Stream{
				URLs:    []downloader.URL{urlData},
				Size:    size,
				Quality: quality,
			}
		}

		for itag, f := range streams {
			if strings.Contains(f.Quality, "video/") {
				if f.URLs[0].Ext == "webm" {
					f.Size += audioWebm.Size
					f.URLs = append(f.URLs, audioWebm)
				} else {
					f.Size += audio.Size
					f.URLs = append(f.URLs, audio)
				}
				streams[itag] = f
			}
		}

		captionURL := ""
		if len(anotherPlayerResponse.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks) > 0 {
			captionURL = anotherPlayerResponse.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks[0].BaseURL
		}

		// outStr, _ := json.Marshal(streams)
		// ioutil.WriteFile("downloader_data.json", outStr, 0644)

		return downloader.Data{
			Site:       "YouTube youtube.com",
			Title:      anotherPlayerResponse.VideoDetails.Title,
			Type:       "video",
			Streams:    streams,
			URL:        uri,
			Thumbnail:  anotherPlayerResponse.VideoDetails.Thumbnail.Thumbnails[1].URL,
			Length:     anotherPlayerResponse.VideoDetails.LengthSeconds,
			CaptionURL: captionURL,
		}, nil
		// }
	}
	return downloader.Data{}, errors.New("can not parse video")
}

func getBaseJS(cacheJL *cache.Cache) string {
	baseJS, found := cacheJL.Get(generalBaseJSURLKey)
	if !found {
		return generalBaseJSURL
	}
	return baseJS.(string)
}
