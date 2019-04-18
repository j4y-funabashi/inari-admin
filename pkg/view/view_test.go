package view_test

import (
	"testing"
	"time"

	"github.com/j4y_funabashi/inari-admin/pkg/view"
	"github.com/matryer/is"
)

func TestItFormatsDates(t *testing.T) {

	is := is.New(t)

	// create valid media item
	now, err := time.Parse(time.RFC3339, "2019-01-28T13:00:00Z")
	if err != nil {
		t.Errorf("Failed to parse time:: %s", err.Error())
	}
	media := view.MediaItem{
		DateTime: &now,
		Lat:      53.800968166666664,
		Lng:      -1.5413559444444442,
	}

	is.Equal(
		"Mon, Jan 28, 2019 13:00 +0000",
		media.HumanDate(),
	)
	is.Equal(
		"2019-01-28T13:00:00Z",
		media.MachineDate(),
	)
	is.True(media.HasLocation())
}
