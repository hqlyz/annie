package downloader

import (
	"encoding/xml"
	"fmt"
	"sort"

	"github.com/fatih/color"

	"github.com/hqlyz/annie/myconfig"
)

// URL data struct for single URL information
type URL struct {
	URL  string `json:"url"`
	Size int64  `json:"size"`
	Ext  string `json:"ext"`
}

// Stream data struct for each stream
type Stream struct {
	// [URL: {URL, Size, Ext}, ...]
	// Some video files have multiple fragments
	// and support for downloading multiple image files at once
	URLs    []URL  `json:"urls"`
	Quality string `json:"quality"`
	// total size of all urls
	Size int64 `json:"size"`

	// name used in sortedStreams
	name string
	Itag string `json:"name"`
}

// Data data struct for video information
type Data struct {
	Site  string `json:"site"`
	Title string `json:"title"`
	Type  string `json:"type"`
	// each stream has it's own URLs and Quality
	Streams       map[string]Stream `json:"streams"`
	sortedStreams []Stream

	// Err is used to record whether an error occurred when extracting data.
	// It is used to record the error information corresponding to each url when extracting the list data.
	// NOTE(hqlyz): err is only used in Data list
	Err error `json:"-"`
	// URL is used to record the address of this download
	URL                 string `json:"url"`
	OutputSortedStreams []Stream
	Thumbnail           string `json:"thumbnail"`
	ThumbnailWidth      int    `json:"twidth"`
	ThumbnailHeight     int    `json:"theight"`
	Length              string `json:"length"`
	CaptionURL          string `json:"caption_url"`
}

// OutputData for api
type OutputData struct {
	Title           string         `json:"title"`
	URL             string         `json:"url"`
	Streams         []OutputStream `json:"streams"`
	Thumbnail       string         `json:"thumbnail"`
	ThumbnailWidth  int            `json:"twidth"`
	ThumbnailHeight int            `json:"theight"`
	Length          string         `json:"length"`
}

// OutputStream for api
type OutputStream struct {
	Quality        string `json:"quality"`
	Type           string `json:"type"`
	Size           int64  `json:"size"`
	DownloadParams string `json:"params"`
	QParams        string `json:"qparams"`
}

/*******************/
// srt struct
// Transcript -
type Transcript struct {
	XMLName xml.Name `xml:"transcript"`
	Texts   []Text   `xml:"text"`
}

// Text -
type Text struct {
	XMLName  xml.Name `xml:"text"`
	Start    xml.Attr `xml:"start,attr"`
	Duration xml.Attr `xml:"dur,attr"`
	Content  string   `xml:",innerxml"`
}

/*******************/

// EmptyList empty Data list
var EmptyList = make([]Data, 0)

// EmptyData returns an "empty" Data object with the given URL and error
func EmptyData(url string, err error) Data {
	return Data{
		URL: url,
		Err: err,
	}
}

func (data *Stream) calculateTotalSize() {
	var size int64
	for _, urlData := range data.URLs {
		size += urlData.Size
	}
	data.Size = size
}

func (data Stream) printStream() {
	blue := color.New(color.FgBlue)
	cyan := color.New(color.FgCyan)
	blue.Println(fmt.Sprintf("     [%s]  -------------------", data.name))
	if data.Quality != "" {
		cyan.Printf("     Quality:         ")
		fmt.Println(data.Quality)
	}
	cyan.Printf("     Size:            ")
	if data.Size == 0 {
		data.calculateTotalSize()
	}
	fmt.Printf("%.2f MiB (%d Bytes)\n", float64(data.Size)/(1024*1024), data.Size)
	cyan.Printf("     # download with: ")
	fmt.Printf("annie -f %s ...\n\n", data.name)
}

func (v *Data) genSortedStreams() {
	for k, data := range v.Streams {
		if data.Size == 0 {
			data.calculateTotalSize()
		}
		data.name = k
		v.Streams[k] = data
		v.sortedStreams = append(v.sortedStreams, data)
	}
	if len(v.Streams) > 1 {
		sort.Slice(
			v.sortedStreams, func(i, j int) bool { return v.sortedStreams[i].Size > v.sortedStreams[j].Size },
		)
	}
}

// SortStreams for api
func (v *Data) SortStreams() {
	for k, data := range v.Streams {
		if data.Size == 0 {
			data.calculateTotalSize()
		}
		data.Itag = k
		v.Streams[k] = data
		v.OutputSortedStreams = append(v.OutputSortedStreams, data)
	}
	if len(v.Streams) > 1 {
		sort.Slice(
			v.OutputSortedStreams, func(i, j int) bool { return v.OutputSortedStreams[i].Size > v.OutputSortedStreams[j].Size },
		)
	}
}

func (v *Data) printInfo(stream string, config myconfig.Config) {
	cyan := color.New(color.FgCyan)
	fmt.Println()
	cyan.Printf(" Site:      ")
	fmt.Println(v.Site)
	cyan.Printf(" Title:     ")
	fmt.Println(v.Title)
	cyan.Printf(" Type:      ")
	fmt.Println(v.Type)
	if config.InfoOnly {
		cyan.Printf(" Streams:   ")
		fmt.Println("# All available quality")
		for _, data := range v.sortedStreams {
			data.printStream()
		}
	} else {
		cyan.Printf(" Stream:   ")
		fmt.Println()
		v.Streams[stream].printStream()
	}
}

// Aria2RPCData json RPC 2.0 for Aria2
type Aria2RPCData struct {
	// More info about RPC interface please refer to
	// https://aria2.github.io/manual/en/html/aria2c.html#rpc-interface
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	// For a simple download, only inplemented `addUri`
	Method string `json:"method"`
	// secret, uris, options
	Params [3]interface{} `json:"params"`
}

// Aria2Input options for `aria2.addUri`
// https://aria2.github.io/manual/en/html/aria2c.html#id3
type Aria2Input struct {
	// The file name of the downloaded file
	Out string `json:"out"`
	// For a simple download, only add headers
	Header []string `json:"header"`
}
