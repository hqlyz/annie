package downloader

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/cheggaaa/pb"
	"github.com/hqlyz/annie/myconfig"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
	"github.com/patrickmn/go-cache"
)

var wg = sync.WaitGroup{}
var mBytes = 1024 * 1024

func progressBar(size int64) *pb.ProgressBar {
	bar := pb.New64(size).SetUnits(pb.U_BYTES).SetRefreshRate(time.Millisecond * 10)
	bar.ShowSpeed = true
	bar.ShowFinalTime = true
	bar.SetMaxWidth(1000)
	return bar
}

// Caption download danmaku, subtitles, etc
func Caption(url, refer, fileName, ext string, config myconfig.Config) error {
	if !config.Caption || config.InfoOnly {
		return nil
	}
	fmt.Println("\nDownloading captions...")
	body, err := request.Get(url, refer, nil, config)
	if err != nil {
		return err
	}
	filePath, err := utils.FilePath(fileName, ext, true, config)
	if err != nil {
		return err
	}
	file, fileError := os.Create(filePath)
	if fileError != nil {
		return fileError
	}
	defer file.Close()
	file.WriteString(body)
	return nil
}

func writeFile(destURL string, file *os.File, headers map[string]string, bar *pb.ProgressBar, cacheJL *cache.Cache, token string, config myconfig.Config) (int64, error) {
	var (
		res *http.Response
		err error
	)
	cacheJL.Set(token+"d", int64(0), time.Minute*10)
	res, err = request.Request(http.MethodGet, destURL, nil, headers, config)
	if err != nil {
		fmt.Println(err.Error())
		return 0, err
	}
	defer res.Body.Close()
	l := res.Header.Get("Content-Length")
	length, _ := strconv.ParseInt(l, 10, 64)
	var goroutineNum int
	if length <= int64(10*mBytes) {
		goroutineNum = int(math.Ceil(float64(length) / float64(mBytes)))
	} else if length <= int64(100*mBytes) {
		goroutineNum = 10
	} else if length <= int64(200*mBytes) {
		goroutineNum = 18
	} else if length <= int64(300*mBytes) {
		goroutineNum = 26
	} else if length <= int64(400*mBytes) {
		goroutineNum = 34
	} else {
		goroutineNum = 42
	}
	fragmentSize := int64(math.Ceil(float64(length) / float64(goroutineNum)))
	wg.Add(goroutineNum)
	var (
		fNum int64
		errs []error
	)
	for i := 0; i < goroutineNum; i++ {
		fileName := file.Name() + strconv.Itoa(i)
		if i == (goroutineNum - 1) {
			fNum = length - (int64(goroutineNum-1))*fragmentSize
		} else {
			fNum = fragmentSize
		}
		seek := fragmentSize * int64(i)
		ranges := fmt.Sprintf("bytes=%d-%d", seek, seek+fNum-1)
		header := make(map[string]string)
		for k, v := range headers {
			header[k] = v
		}
		header["Range"] = ranges
		go func(destURL string, fileName string, header map[string]string, cacheJL *cache.Cache, token string, config myconfig.Config) {
			err := fragmentDownload(destURL, fileName, header, cacheJL, token, config)
			if err != nil {
				errs = append(errs, err)
			}
		}(destURL, fileName, header, cacheJL, token, config)
	}
	wg.Wait()

	// merge files
	fmt.Printf("error count: %d\n", len(errs))
	if len(errs) > 0 {
		fmt.Println(errs[0].Error())
		return 0, errs[0]
	}
	for i := 0; i < goroutineNum; i++ {
		tempFile, err := os.Open(file.Name() + strconv.Itoa(i))
		if err != nil {
			return 0, err
		}
		seek := int64(i) * fragmentSize
		file.Seek(seek, 0)
		_, err = io.Copy(file, tempFile)
		if err != nil {
			fmt.Println(err.Error())
			return 0, err
		}
		tempFile.Close()
		err = os.Remove(file.Name() + strconv.Itoa(i))
		if err != nil {
			return 0, err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return length, nil
}

func fragmentDownload(destURL string, fileName string, headers map[string]string, cacheJL *cache.Cache, token string, config myconfig.Config) error {
	defer wg.Done()
	client := grab.NewClient()
	if config.Proxy != "" {
		httpProxy, err := url.Parse(config.Proxy)
		if err != nil {
			return err
		}
		client.HTTPClient = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(httpProxy),
			},
		}
	}
	req, _ := grab.NewRequest(fileName, destURL)
	for k, v := range headers {
		req.HTTPRequest.Header.Set(k, v)
	}

	fi, err := os.Stat(fileName)
	resp := client.Do(req)
	// if err != nil {
	// 	fmt.Printf("fi.Size(): 0\n")
	// } else {
	// 	fmt.Printf("fi.Size(): %d\n", fi.Size())
	// }
	// fmt.Printf("resp.Size(): %d\n", resp.Size())
	// if file already exsits, skip downloading
	if err == nil && fi.Size() == resp.Size() {
		fmt.Println("the file already exsits")
		resp.Cancel()
		return nil
	}
	t := time.NewTicker(500 * time.Millisecond)
	timeout := time.After(time.Minute * 10)
	defer t.Stop()
	var (
		preBytesComplete = int64(0)
		bytesComplete    = int64(0)
	)
Loop:
	for {
		select {
		case <-t.C:
			bytesComplete = resp.BytesComplete()
			cacheJL.Increment(token+"d", bytesComplete-preBytesComplete)
			preBytesComplete = bytesComplete
		case <-resp.Done:
			if resp.BytesComplete()-preBytesComplete > 0 {
				cacheJL.Increment(token+"d", resp.BytesComplete()-preBytesComplete)
			}
			break Loop
		case <-timeout:
			fmt.Println("Download timeout")
			return errors.New("Download timeout")
		}

	}

	if err := resp.Err(); err != nil {
		fmt.Printf("Download failed: %v\n", err)
		return err
	}
	return nil
}

/* original version */
// func writeFile(
// 	url string, file *os.File, headers map[string]string, bar *pb.ProgressBar,
// ) (int64, error) {
// 	res, err := request.Request("GET", url, nil, headers)
// 	if err != nil {
// 		return 0, err
// 	}
// 	defer res.Body.Close()
// 	writer := io.MultiWriter(file, bar)
// 	// Note that io.Copy reads 32kb(maximum) from input and writes them to output, then repeats.
// 	// So don't worry about memory.
// 	written, copyErr := io.Copy(writer, res.Body)
// 	if copyErr != nil {
// 		return written, fmt.Errorf("file copy error: %s", copyErr)
// 	}
// 	return written, nil
// }

// Save save url file
func Save(urlData URL, refer, fileName string, bar *pb.ProgressBar, chunkSizeMB int, cacheJL *cache.Cache, token string, config myconfig.Config) error {
	var err error
	filePath, err := utils.FilePath(fileName, urlData.Ext, true, config)
	if err != nil {
		return err
	}
	// filePath = strings.Replace(filePath, "\\", " ", -1)
	fileSize, exists, err := utils.FileSize(filePath)
	if err != nil {
		return err
	}
	// if bar == nil {
	// 	bar = progressBar(urlData.Size)
	// 	bar.Start()
	// }
	// Skip segment file
	// TODO: Live video URLs will not return the size
	if exists && fileSize == urlData.Size {
		// bar.Add64(fileSize)
		return nil
	}
	tempFilePath := filePath + ".download"
	tempFileSize, _, err := utils.FileSize(tempFilePath)
	if err != nil {
		return err
	}
	headers := map[string]string{
		"Referer": refer,
	}
	var (
		file      *os.File
		fileError error
	)
	if tempFileSize > 0 {
		// range start from 0, 0-1023 means the first 1024 bytes of the file
		headers["Range"] = fmt.Sprintf("bytes=%d-", tempFileSize)
		file, fileError = os.OpenFile(tempFilePath, os.O_APPEND|os.O_WRONLY, 0644)
		// bar.Add64(tempFileSize)
	} else {
		// file, fileError = os.Create(tempFilePath)
		file, fileError = os.OpenFile(tempFilePath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	}
	if fileError != nil {
		return fileError
	}
	if chunkSizeMB > 0 {
		var start, end, chunkSize int64
		chunkSize = int64(chunkSizeMB) * 1024 * 1024
		remainingSize := urlData.Size
		if tempFileSize > 0 {
			start = tempFileSize
			remainingSize -= tempFileSize
		}
		chunk := remainingSize / chunkSize
		if remainingSize%chunkSize != 0 {
			chunk++
		}
		var i int64 = 1
		for ; i <= chunk; i++ {
			end = start + chunkSize - 1
			headers["Range"] = fmt.Sprintf("bytes=%d-%d", start, end)
			temp := start
			for i := 0; ; i++ {
				written, err := writeFile(urlData.URL, file, headers, bar, cacheJL, token, config)
				if err == nil {
					break
				} else if i+1 >= config.RetryTimes {
					return err
				}
				temp += written
				headers["Range"] = fmt.Sprintf("bytes=%d-%d", temp, end)
				time.Sleep(1 * time.Second)
			}
			start = end + 1
		}
	} else {
		temp := tempFileSize
		for i := 0; ; i++ {
			written, err := writeFile(urlData.URL, file, headers, bar, cacheJL, token, config)
			if err == nil {
				break
			} else if i+1 >= config.RetryTimes {
				return err
			}
			temp += written
			headers["Range"] = fmt.Sprintf("bytes=%d-", temp)
			time.Sleep(1 * time.Second)
		}
	}

	// close and rename temp file at the end of this function
	defer func() {
		// must close the file before rename or it will cause
		// `The process cannot access the file because it is being used by another process.` error.
		file.Close()
		if err == nil {
			os.Rename(tempFilePath, filePath)
		}
	}()
	return nil
}

// Download download urls
func Download(v Data, refer string, chunkSizeMB int, cacheJL *cache.Cache, token string, config myconfig.Config) error {
	v.genSortedStreams()
	if config.ExtractedData {
		jsonData, _ := json.MarshalIndent(v, "", "    ")
		fmt.Printf("%s\n", jsonData)
		return nil
	}
	var (
		title  string
		stream string
	)
	if config.OutputName == "" {
		title = utils.FileName(v.Title)
	} else {
		title = utils.FileName(config.OutputName)
	}
	if config.Stream == "" {
		stream = v.sortedStreams[0].name
	} else {
		stream = config.Stream
	}
	data, ok := v.Streams[stream]
	if !ok {
		return fmt.Errorf("no stream named %s", stream)
	}
	title = title + " - " + splitVideoQuality(data.Quality)
	v.printInfo(stream, config) // if InfoOnly, this func will print all streams info
	if config.InfoOnly {
		return nil
	}

	// Use aria2 rpc to download
	if config.UseAria2RPC {
		rpcData := Aria2RPCData{
			JSONRPC: "2.0",
			ID:      "annie", // can be modified
			Method:  "aria2.addUri",
		}
		rpcData.Params[0] = "token:" + config.Aria2Token
		var urls []string
		for _, p := range data.URLs {
			urls = append(urls, p.URL)
		}
		var inputs Aria2Input
		inputs.Header = append(inputs.Header, "Referer: "+refer)
		for i := range urls {
			rpcData.Params[1] = urls[i : i+1]
			inputs.Out = fmt.Sprintf("%s[%d].%s", title, i, data.URLs[0].Ext)
			rpcData.Params[2] = &inputs
			jsonData, err := json.Marshal(rpcData)
			if err != nil {
				return err
			}
			reqURL := fmt.Sprintf("%s://%s/jsonrpc", config.Aria2Method, config.Aria2Addr)
			req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			var client http.Client
			_, err = client.Do(req)
			if err != nil {
				return err
			}
		}
		return nil
	}
	var err error
	// Skip the complete file that has been merged
	// mergedFilePath, err := utils.FilePath(title, "mp4", false, config)
	mergedFilePath, err := utils.FilePath(title, data.URLs[0].Ext, false, config)
	if err != nil {
		return err
	}
	_, mergedFileExists, err := utils.FileSize(mergedFilePath)
	if err != nil {
		return err
	}
	// After the merge, the file size has changed, so we do not check whether the size matches
	var convertCacheValue string
	if len(data.URLs) == 1 {
		convertCacheValue = config.OutputPath + "/" + strings.Replace(title, "/", " ", -1) + "." + data.URLs[0].Ext
	} else {
		convertCacheValue = mergedFilePath
	}

	cacheJL.Set(token+"c", convertCacheValue, time.Minute*60)
	cacheJL.Set(token+"d", int64(0), time.Minute*60)
	if mergedFileExists {
		fmt.Printf("%s: file already exists, skipping\n", mergedFilePath)
		cacheJL.Set(token+"d", data.Size, time.Minute*60)
		tryToDownloadSrt(v, config, title, cacheJL, token)
		return nil
	}
	bar := progressBar(data.Size)
	// bar.Start()
	if len(data.URLs) == 1 {
		// only one fragment
		cacheJL.Set(token+"c", convertCacheValue, time.Minute*60)
		err := Save(data.URLs[0], refer, title, bar, chunkSizeMB, cacheJL, token, config)
		if err != nil {
			return err
		}
		// bar.Finish()
		return nil
	}
	wgp := utils.NewWaitGroupPool(config.ThreadNumber)
	// multiple fragments
	errs := make([]error, 0)
	parts := make([]string, len(data.URLs))
	// try to download video caption
	go tryToDownloadSrt(v, config, title, cacheJL, token)
	for index, url := range data.URLs {
		partFileName := fmt.Sprintf("%s[%d]", title, index)
		partFilePath, err := utils.FilePath(partFileName, url.Ext, false, config)
		if err != nil {
			return err
		}
		parts[index] = partFilePath

		wgp.Add()
		go func(url URL, refer, fileName string, bar *pb.ProgressBar) {
			defer wgp.Done()
			err := Save(url, refer, fileName, bar, chunkSizeMB, cacheJL, token, config)
			if err != nil {
				errs = append(errs, err)
			}
		}(url, refer, partFileName, bar)
	}
	wgp.Wait()
	if len(errs) > 0 {
		return errs[0]
	}
	// bar.Finish()

	if v.Type != "video" {
		return nil
	}
	// merge
	fmt.Printf("Merging video parts into %s\n", mergedFilePath)
	if v.Site == "YouTube youtube.com" {
		err = utils.MergeAudioAndVideo(parts, mergedFilePath)
	} else {
		err = utils.MergeToMP4(parts, mergedFilePath, title)
	}
	if err != nil {
		return err
	}

	return nil
}

func splitVideoQuality(quality string) string {
	return strings.Split(quality, " ")[0]
}

func tryToDownloadSrt(v Data, config myconfig.Config, title string, cacheJL *cache.Cache, token string) {
	fmt.Println("try to download video caption")
	if v.CaptionURL == "" {
		fmt.Println("the caption url is empty, skip downloading caption")
		return
	}
	// fmt.Printf("the caption url is: %s\n", v.CaptionURL)
	srtPath, err := utils.FilePath(title, "srt", false, config)
	if _, err := os.Stat(srtPath); err == nil {
		cacheJL.Set(token+"cs", srtPath, time.Hour*1)
		return
	}
	captionHTML, err := request.Get(v.CaptionURL, "", nil, config)
	// fmt.Printf("the caption text: %s\n", captionHTML)
	if err != nil {
		fmt.Printf("get caption error: %s\n", err.Error())
		return
	}
	downloadSrt(captionHTML, srtPath, cacheJL, token)
}

func downloadSrt(str string, name string, cacheJL *cache.Cache, token string) {
	// ioutil.WriteFile("yt_srt.txt", []byte(str), 0644)
	var caption Transcript
	err := xml.Unmarshal([]byte(str), &caption)
	if err != nil {
		return
	}
	outputStr := ""
	for k, v := range caption.Texts {
		tempStr := fmt.Sprintf("%d\n", k+1)
		start, err := strconv.ParseFloat(v.Start.Value, 64)
		if err != nil {
			return
		}
		dur, err := strconv.ParseFloat(v.Duration.Value, 64)
		if err != nil {
			return
		}
		end := start + dur
		m, _ := divMod(int(start), 60)
		h, m := divMod(m, 60)
		startStr := strings.Replace(fmt.Sprintf("%02d:%02d:%06.3f", h, m, (start-float64(h)*3600-float64(m)*60)), ".", ",", -1)
		m, _ = divMod(int(end), 60)
		h, m = divMod(m, 60)
		endStr := strings.Replace(fmt.Sprintf("%02d:%02d:%06.3f", h, m, (end-float64(h)*3600-float64(m)*60)), ".", ",", -1)
		tempStr += fmt.Sprintf("%s --> %s\n%s\n\n", startStr, endStr, strings.Replace(v.Content, "&amp;#39;", "'", -1))
		outputStr += tempStr
	}
	ioutil.WriteFile(name, []byte(outputStr), 0644)
	cacheJL.Set(token+"cs", name, time.Hour*1)
}

func divMod(x int, y int) (int, int) {
	div := x / y
	mod := x % y
	return div, mod
}
