package geekbang

import (
	"testing"

	"github.com/hqlyz/annie/myconfig"
	"github.com/hqlyz/annie/test"
)

func TestDownload(t *testing.T) {
	config.InfoOnly = true
	config.RetryTimes = 100
	tests := []struct {
		name string
		args test.Args
	}{
		{
			name: "normal test",
			args: test.Args{
				URL:   "https://time.geekbang.org/course/detail/190-97203",
				Title: "02 | 内容综述 - 玩转webpack",
				Size:  38556544,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Extract(tt.args.URL)
			test.CheckError(t, err)
			test.Check(t, tt.args, data[0])
		})
	}
}
