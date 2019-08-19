package view_test

import (
	"testing"
	"time"

	"github.com/j4y_funabashi/inari-admin/pkg/okami"
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

func TestParseMediaListViewModel(t *testing.T) {

	is := is.New(t)

	mediaDat1, _ := time.Parse(time.RFC3339, "2006-01-28T15:04:05Z")
	mediaDat2, _ := time.Parse(time.RFC3339, "2007-01-28T15:04:05Z")

	// arrange
	mediaResponse := okami.ListMediaResponse{
		CurrentYear:  "2019",
		CurrentMonth: "3",
		Months: []okami.ArchiveMonth{
			okami.ArchiveMonth{Month: "2", Count: 109},
			okami.ArchiveMonth{Month: "10", Count: 10},
		},
		Years: []okami.ArchiveYear{
			okami.ArchiveYear{Year: "2019", Count: 1},
			okami.ArchiveYear{Year: "2016", Count: 4},
		},
		Media: []okami.Media{
			okami.Media{URL: "http://example.com/1.jpg", DateTime: &mediaDat1, IsPublished: true},
			okami.Media{URL: "http://example.com/2.jpg", DateTime: &mediaDat2, IsPublished: false},
			okami.Media{URL: "http://example.com/3.jpg", DateTime: &mediaDat2, IsPublished: false},
		},
		AfterKey: "test-after-key",
	}

	expected := view.ListMediaView{
		Months: []view.Month{
			view.Month{Month: "February, 2019", Count: 109, Link: "?month=2&year=2019"},
			view.Month{Month: "October, 2019", Count: 10, Link: "?month=10&year=2019"},
		},
		Years: []view.Year{
			view.Year{Year: "2019", Count: 1, Link: "?year=2019"},
			view.Year{Year: "2016", Count: 4, Link: "?year=2016"},
		},
		CurrentMonth: "March",
		CurrentYear:  "2019",
		Media: []view.Media{
			view.Media{URL: "http://example.com/1.jpg", IsPublished: true},
			view.Media{URL: "http://example.com/2.jpg", IsPublished: false},
			view.Media{
				URL:         "http://example.com/3.jpg",
				IsPublished: false,
				HumanDate:   "Sat, Jan 28, 2006 15:04 +0000",
				MachineDate: "2006-01-28T15:04:05Z",
				Lat:         0,
				Lng:         0,
			},
		},
		AfterKey:  "test-after-key",
		HasPaging: true,
		PageTitle: "Choose some shiz to shizzle with",
		MediaDays: []view.MediaDay{
			view.MediaDay{
				Date: "Sat, 28 January",
				Media: []view.Media{
					view.Media{URL: "http://example.com/1.jpg"},
				},
				Count: 1,
			},
			view.MediaDay{
				Date: "Sun, 28 January",
				Media: []view.Media{
					view.Media{URL: "http://example.com/2.jpg"},
					view.Media{URL: "http://example.com/3.jpg"},
				},
				Count: 2,
			},
		},
	}

	// act
	result := view.ParseListMediaView(mediaResponse)

	// assert
	is.Equal(result.Media, expected.Media)

}
