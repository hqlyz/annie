package pornhub

import (
	"testing"

	"github.com/hqlyz/annie/myconfig"
	"github.com/hqlyz/annie/test"
)

var config = myconfig.New()

func TestPornhub(t *testing.T) {
	config.InfoOnly = true
	config.RetryTimes = 10
	tests := []struct {
		name string
		args test.Args
	}{
		{
			name: "normal test",
			args: test.Args{
				URL:     "https://www.pornhub.com/view_video.php?viewkey=ph5cb5fc41c6ebd",
				Title:   "Must watch Milf drilled by the fireplace",
				Quality: "720P",
				Size:    158868371,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Extract(tt.args.URL, config)
			test.CheckError(t, err)
			test.Check(t, tt.args, data[0])
		})
	}
}
