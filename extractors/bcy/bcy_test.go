package bcy

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
				URL:   "https://bcy.net/coser/detail/143767/2094010",
				Title: "phx：柠檬先行预告！牧濑红莉栖 cn: 三度 - 半次元 - ACG爱好者社区",
				Size:  3329959,
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
