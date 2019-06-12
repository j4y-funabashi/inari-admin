package okami_test

import (
	"testing"
	"time"

	"github.com/j4y_funabashi/inari-admin/pkg/mf2"
	"github.com/j4y_funabashi/inari-admin/pkg/mpclient"
	"github.com/j4y_funabashi/inari-admin/pkg/okami"
	"github.com/matryer/is"
	"github.com/sirupsen/logrus"
)

func TestListMedia(t *testing.T) {
	var tests = []struct {
		name string
	}{
		{name: "it works"},
	}

	for _, tt := range tests {
		is := is.NewRelaxed(t)
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			// arrange
			mockMpClient := MockMPClient{}
			logger := logrus.New()
			app := okami.New(mockMpClient, logger)

			micropubEndpoint := ""
			accessToken := ""
			afterKey := ""
			selectedYear := ""
			selectedMonth := ""
			mediaDat1, _ := time.Parse(time.RFC3339, "2006-01-28T15:04:05Z")
			expectedYears := []okami.ArchiveYear{
				okami.ArchiveYear{Year: "2019", Count: 1},
				okami.ArchiveYear{Year: "2018", Count: 2},
				okami.ArchiveYear{Year: "2015", Count: 3},
			}
			expectedMonths := []okami.ArchiveMonth{
				okami.ArchiveMonth{Month: "09", Count: 1},
				okami.ArchiveMonth{Month: "10", Count: 1},
			}
			expectedMedia := []okami.Media{
				okami.Media{URL: "http://media.example.com/1", IsPublished: true, DateTime: &mediaDat1},
				okami.Media{URL: "http://media.example.com/2", IsPublished: false, DateTime: &mediaDat1},
			}
			expectedResult := okami.ListMediaResponse{
				Years:        expectedYears,
				Media:        expectedMedia,
				Months:       expectedMonths,
				AfterKey:     "test-after-key-123",
				CurrentYear:  "2019",
				CurrentMonth: "09",
			}

			// act
			result := app.ListMedia(
				micropubEndpoint,
				accessToken,
				afterKey,
				selectedYear,
				selectedMonth,
			)

			// assert
			is.Equal(result, expectedResult)
		})
	}
}

type MockMPClient struct {
}

func (cl MockMPClient) QueryMonthsList(micropubEndpoint, accessToken, currentYear string) ([]mf2.ArchiveMonth, error) {
	return []mf2.ArchiveMonth{
		mf2.ArchiveMonth{Month: "09", Count: 1},
		mf2.ArchiveMonth{Month: "10", Count: 1},
	}, nil
}

func (cl MockMPClient) QueryYearsList(micropubEndpoint, accessToken string) ([]mf2.ArchiveYear, error) {
	return []mf2.ArchiveYear{
		mf2.ArchiveYear{Year: "2019", Count: 1},
		mf2.ArchiveYear{Year: "2018", Count: 2},
		mf2.ArchiveYear{Year: "2015", Count: 3},
	}, nil
}

func (cl MockMPClient) QueryMediaList(mediaEndpoint, accessToken, afterKey, year, month string) (mpclient.MediaQueryListResponse, error) {
	mediaDat1, _ := time.Parse(time.RFC3339, "2006-01-28T15:04:05Z")
	paging := mpclient.ListPaging{
		After: "test-after-key-123",
	}
	return mpclient.MediaQueryListResponse{
		Items: []mpclient.MediaQueryListResponseItem{
			mpclient.MediaQueryListResponseItem{URL: "http://media.example.com/1", IsPublished: true, DateTime: &mediaDat1},
			mpclient.MediaQueryListResponseItem{URL: "http://media.example.com/2", IsPublished: false, DateTime: &mediaDat1},
		},
		Paging: &paging,
	}, nil
}
