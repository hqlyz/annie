package downloader

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/cheggaaa/pb"
	"github.com/hqlyz/annie/myconfig"
	"github.com/hqlyz/annie/request"
	"github.com/hqlyz/annie/utils"
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

func writeFile(url string, file *os.File, headers map[string]string, bar *pb.ProgressBar, cacheJL *cache.Cache, token string, config myconfig.Config) (int64, error) {
	var (
		res *http.Response
		err error
	)
	// cacheJL.Set(token+"d", int64(0), time.Minute*10)
	res, err = request.Request("GET", url, nil, headers, config)
	if err != nil {
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
		goroutineNum = 20
	} else if length <= int64(300*mBytes) {
		goroutineNum = 30
	} else if length <= int64(400*mBytes) {
		goroutineNum = 40
	} else {
		goroutineNum = 50
	}
	fragmentSize := int64(math.Ceil(float64(length) / float64(goroutineNum)))
	wg.Add(goroutineNum)
	var fNum int64
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
		go fragmentDownload(url, header, nil, fileName, cacheJL, token, config)
	}
	wg.Wait()
	// num, found := cacheJL.Get(token)
	// if !found {
	// 	num = -1
	// }
	// fmt.Printf("Download: %d\n", num)

	// merge files
	if err != nil {
		fmt.Println(err.Error())
		return 0, err
	}
	for i := 0; i < goroutineNum; i++ {
		tempFile, err := os.Open(file.Name() + strconv.Itoa(i))
		fileInfo, _ := tempFile.Stat()
		fmt.Printf("file size: %d\n", fileInfo.Size())
		if err != nil {
			return 0, err
		}
		seek := int64(i) * fragmentSize
		file.Seek(seek, 0)
		io.Copy(file, tempFile)
		tempFile.Close()
		err = os.Remove(file.Name() + strconv.Itoa(i))
		if err != nil {
			return 0, err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return length, nil
}

func fragmentDownload(destURL string, headers map[string]string, bar *pb.ProgressBar, fileName string, cacheJL *cache.Cache, token string, config myconfig.Config) {
	var (
		res *http.Response
		err error
	)
	res, err = request.Request("GET", destURL, nil, headers, config)
	if err != nil {
		return
	}
	defer res.Body.Close()
	file, err := os.Create(fileName)
	if err != nil {
		return
	}
	buffer := make([]byte, 4*1024)
	myReader := bufio.NewReader(res.Body)
	myWriter := bufio.NewWriter(file)
	var (
		n int
	)
	for n, err = 0, error(nil); err == nil && err != io.EOF; {
		n, err = myReader.Read(buffer)
		myWriter.Write(buffer[:n])
		cacheJL.Increment(token+"d", int64(n))
	}
	myWriter.Flush()
	file.Close()
	if err != nil && err != io.EOF {
		return
	}
	defer wg.Done()
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
	filePath, err := utils.FilePath(fileName, urlData.Ext, false, config)
	if err != nil {
		return err
	}
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
	mergedFilePath, err := utils.FilePath(title, "mp4", false, config)
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
		convertCacheValue = config.OutputPath + "/" + title + "." + data.URLs[0].Ext
	} else {
		convertCacheValue = mergedFilePath
	}

	cacheJL.Set(token+"c", convertCacheValue, time.Minute*60)
	if mergedFileExists {
		fmt.Printf("%s: file already exists, skipping\n", mergedFilePath)
		cacheJL.Set(token+"d", data.Size, time.Minute*10)
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
