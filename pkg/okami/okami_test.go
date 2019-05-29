package okami_test

import (
	"testing"

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
			micropubEndpoint := ""
			accessToken := ""
			afterKey := ""
			mockMpClient := MockMPClient{}
			logger := logrus.New()
			app := okami.New(mockMpClient, logger)
			expectedYears := []okami.ArchiveYear{
				okami.ArchiveYear{Year: "2019", Count: 1},
				okami.ArchiveYear{Year: "2018", Count: 2},
				okami.ArchiveYear{Year: "2015", Count: 3},
			}
			expectedMedia := []okami.Media{
				okami.Media{URL: "http://media.example.com/1", IsPublished: true},
				okami.Media{URL: "http://media.example.com/2", IsPublished: false},
			}
			expectedResult := okami.ListMediaResponse{
				Years:    expectedYears,
				Media:    expectedMedia,
				AfterKey: "test-after-key-123",
			}

			// act
			result := app.ListMedia(micropubEndpoint, accessToken, afterKey)

			// assert
			is.Equal(result, expectedResult)
		})
	}
}

type MockMPClient struct {
}

func (cl MockMPClient) QueryYearsList(micropubEndpoint, accessToken string) ([]mf2.ArchiveYear, error) {
	return []mf2.ArchiveYear{
		mf2.ArchiveYear{Year: "2019", Count: 1},
		mf2.ArchiveYear{Year: "2018", Count: 2},
		mf2.ArchiveYear{Year: "2015", Count: 3},
	}, nil
}

func (cl MockMPClient) QueryMediaList(mediaEndpoint, accessToken, afterKey string) (mpclient.MediaQueryListResponse, error) {
	paging := mpclient.ListPaging{
		After: "test-after-key-123",
	}
	return mpclient.MediaQueryListResponse{
		Items: []mpclient.MediaQueryListResponseItem{
			mpclient.MediaQueryListResponseItem{URL: "http://media.example.com/1", IsPublished: true},
			mpclient.MediaQueryListResponseItem{URL: "http://media.example.com/2", IsPublished: false},
		},
		Paging: &paging,
	}, nil
}