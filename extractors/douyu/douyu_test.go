package douyu

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
				URL:   "https://v.douyu.com/show/l0Q8mMY3wZqv49Ad",
				Title: "每日撸报_每日撸报：有些人死了其实它还可以把你带走_斗鱼视频 - 最6的弹幕视频网站",
				Size:  10558080,
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
