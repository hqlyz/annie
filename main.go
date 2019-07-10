package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/fatih/color"

	"github.com/hqlyz/annie/downloader"
	"github.com/hqlyz/annie/extractors/bcy"
	"github.com/hqlyz/annie/extractors/bilibili"
	"github.com/hqlyz/annie/extractors/douyin"
	"github.com/hqlyz/annie/extractors/douyu"
	"github.com/hqlyz/annie/extractors/facebook"
	"github.com/hqlyz/annie/extractors/geekbang"
	"github.com/hqlyz/annie/extractors/instagram"
	"github.com/hqlyz/annie/extractors/iqiyi"
	"github.com/hqlyz/annie/extractors/mgtv"
	"github.com/hqlyz/annie/extractors/miaopai"
	"github.com/hqlyz/annie/extractors/netease"
	"github.com/hqlyz/annie/extractors/pixivision"
	"github.com/hqlyz/annie/extractors/pornhub"
	"github.com/hqlyz/annie/extractors/qq"
	"github.com/hqlyz/annie/extractors/tumblr"
	"github.com/hqlyz/annie/extractors/twitter"
	"github.com/hqlyz/annie/extractors/universal"
	"github.com/hqlyz/annie/extractors/vimeo"
	"github.com/hqlyz/annie/extractors/weibo"
	"github.com/hqlyz/annie/extractors/yinyuetai"
	"github.com/hqlyz/annie/extractors/youku"
	"github.com/hqlyz/annie/extractors/youtube"
	"github.com/hqlyz/annie/myconfig"
	"github.com/hqlyz/annie/utils"
)

var cache1 *cache.Cache

func init() {
	flag.BoolVar(&myconfig.Debug, "d", false, "Debug mode")
	flag.BoolVar(&myconfig.Version, "v", false, "Show version")
	flag.BoolVar(&myconfig.InfoOnly, "i", false, "Information only")
	flag.StringVar(&myconfig.Cookie, "c", "", "Cookie")
	flag.BoolVar(&myconfig.Playlist, "p", false, "Download playlist")
	flag.StringVar(&myconfig.Refer, "r", "", "Use specified Referrer")
	flag.StringVar(&myconfig.Proxy, "x", "", "HTTP proxy")
	flag.StringVar(&myconfig.Socks5Proxy, "s", "", "SOCKS5 proxy")
	flag.StringVar(&myconfig.Stream, "f", "", "Select specific stream to download")
	flag.StringVar(&myconfig.OutputPath, "o", "", "Specify the output path")
	flag.StringVar(&myconfig.OutputName, "O", "", "Specify the output file name")
	flag.BoolVar(&myconfig.ExtractedData, "j", false, "Print extracted data")
	flag.IntVar(&myconfig.ChunkSizeMB, "cs", 0, "HTTP chunk size for downloading (in MB)")
	flag.BoolVar(&myconfig.UseAria2RPC, "aria2", false, "Use Aria2 RPC to download")
	flag.StringVar(&myconfig.Aria2Token, "aria2token", "", "Aria2 RPC Token")
	flag.StringVar(&myconfig.Aria2Addr, "aria2addr", "localhost:6800", "Aria2 Address")
	flag.StringVar(&myconfig.Aria2Method, "aria2method", "http", "Aria2 Method")
	flag.IntVar(
		&myconfig.ThreadNumber, "n", 10, "The number of download thread (only works for multiple-parts video)",
	)
	flag.StringVar(&myconfig.File, "F", "", "URLs file path")
	flag.IntVar(&myconfig.PlaylistStart, "start", 1, "Playlist video to start at")
	flag.IntVar(&myconfig.PlaylistEnd, "end", 0, "Playlist video to end at")
	flag.StringVar(
		&myconfig.PlaylistItems, "items", "",
		"Playlist video items to download. Separated by commas like: 1,5,6",
	)
	flag.BoolVar(&myconfig.Caption, "C", false, "Download captions")
	flag.IntVar(
		&myconfig.RetryTimes, "retry", 10, "How many times to retry when the download failed",
	)
	// youku
	flag.StringVar(&myconfig.YoukuCcode, "ccode", "0590", "Youku ccode")
	flag.StringVar(
		&myconfig.YoukuCkey,
		"ckey",
		"7B19C0AB12633B22E7FE81271162026020570708D6CC189E4924503C49D243A0DE6CD84A766832C2C99898FC5ED31F3709BB3CDD82C96492E721BDD381735026",
		"Youku ckey",
	)
	flag.StringVar(&myconfig.YoukuPassword, "password", "", "Youku password")
	// youtube
	flag.BoolVar(&myconfig.YouTubeStream2, "ytb-stream2", false, "Use data in url_encoded_fmt_stream_map")
	cache1 = cache.New(time.Hour*2, time.Minute*5)
}

func printError(url string, err error) {
	fmt.Printf(
		"Downloading %s error:\n%s\n",
		color.CyanString("%s", url), color.RedString("%v", err),
	)
}

func download(videoURL string, config myconfig.Config) bool {
	var (
		domain string
		err    error
		data   []downloader.Data
	)
	// mycache.Cache = cache.New(time.Hour*2, time.Minute*5)
	// config := myconfig.New()
	bilibiliShortLink := utils.MatchOneOf(videoURL, `^(av|ep)\d+`)
	if bilibiliShortLink != nil {
		bilibiliURL := map[string]string{
			"av": "https://www.bilibili.com/video/",
			"ep": "https://www.bilibili.com/bangumi/play/",
		}
		domain = "bilibili"
		videoURL = bilibiliURL[bilibiliShortLink[1]] + videoURL
	} else {
		u, err := url.ParseRequestURI(videoURL)
		if err != nil {
			printError(videoURL, err)
			return true
		}
		domain = utils.Domain(u.Host)
	}
	switch domain {
	case "douyin", "iesdouyin":
		data, err = douyin.Extract(videoURL, config)
	case "bilibili":
		data, err = bilibili.Extract(videoURL, config)
	case "bcy":
		data, err = bcy.Extract(videoURL, config)
	case "pixivision":
		data, err = pixivision.Extract(videoURL, config)
	case "youku":
		data, err = youku.Extract(videoURL, config)
	case "youtube", "youtu": // youtu.be
		data, err = youtube.Extract(videoURL, cache1, config)
	case "iqiyi":
		data, err = iqiyi.Extract(videoURL, config)
	case "mgtv":
		data, err = mgtv.Extract(videoURL, config)
	case "tumblr":
		data, err = tumblr.Extract(videoURL, config)
	case "vimeo":
		data, err = vimeo.Extract(videoURL, config)
	case "facebook":
		data, err = facebook.Extract(videoURL, config)
	case "douyu":
		data, err = douyu.Extract(videoURL, config)
	case "miaopai":
		data, err = miaopai.Extract(videoURL, config)
	case "163":
		data, err = netease.Extract(videoURL, config)
	case "weibo":
		data, err = weibo.Extract(videoURL, config)
	case "instagram":
		data, err = instagram.Extract(videoURL, config)
	case "twitter":
		data, err = twitter.Extract(videoURL, config)
	case "qq":
		data, err = qq.Extract(videoURL, config)
	case "yinyuetai":
		data, err = yinyuetai.Extract(videoURL, config)
	case "geekbang":
		data, err = geekbang.Extract(videoURL, config)
	case "pornhub":
		data, err = pornhub.Extract(videoURL, config)
	default:
		data, err = universal.Extract(videoURL, config)
	}
	if err != nil {
		// if this error occurs, it means that an error occurred before actually starting to extract data
		// (there is an error in the preparation step), and the data list is empty.
		printError(videoURL, err)
		return true
	}
	var isErr bool
	for _, item := range data {
		if item.Err != nil {
			// if this error occurs, the preparation step is normal, but the data extraction is wrong.
			// the data is an empty struct.
			printError(item.URL, item.Err)
			isErr = true
			continue
		}
		err = downloader.Download(item, videoURL, config.ChunkSizeMB, cache1, "lalala", config)
		if err != nil {
			printError(item.URL, err)
			isErr = true
		}
	}
	return isErr
}

func main() {
	config := myconfig.New()
	// config.InfoOnly = true
	flag.Parse()
	args := flag.Args()
	if config.Version {
		utils.PrintVersion(config)
		return
	}
	if config.Debug {
		utils.PrintVersion(config)
	}
	if config.File != "" {
		// read URL list from file
		file, err := os.Open(config.File)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			universalURL := strings.TrimSpace(scanner.Text())
			if universalURL == "" {
				continue
			}
			args = append(args, universalURL)
		}
	}
	if len(args) < 1 {
		fmt.Println("Too few arguments")
		fmt.Println("Usage: annie [args] URLs...")
		flag.PrintDefaults()
		return
	}
	if config.Cookie != "" {
		// If config.Cookie is a file path, convert it to a string to ensure
		// config.Cookie is always string
		if _, fileErr := os.Stat(config.Cookie); fileErr == nil {
			// Cookie is a file
			data, err := ioutil.ReadFile(config.Cookie)
			if err != nil {
				color.Red("%v", err)
				return
			}
			config.Cookie = string(data)
		}
	}
	var isErr bool
	for _, videoURL := range args {
		if err := download(strings.TrimSpace(videoURL), config); err {
			isErr = true
		}
	}
	if isErr {
		os.Exit(1)
	}
}
