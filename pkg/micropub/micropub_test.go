package micropub_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/j4y_funabashi/inari-admin/pkg/micropub"
	"github.com/matryer/is"
	"github.com/sirupsen/logrus"
)

func TestMediaQueryByURL(t *testing.T) {

	var tests = []struct {
		name string
	}{
		{"it works"},
	}

	for _, tt := range tests {

		is := is.NewRelaxed(t)
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			// arrange
			accessToken := "test-token"
			URL := "test-url"
			mediaServer := newMediaServerSingleItem(t, getValidMediaItem())
			mediaEndpoint := mediaServer.URL
			logger := logrus.New()
			mpclient := micropub.NewClient(logger)

			// act
			response, err := mpclient.QueryMediaURL(URL, mediaEndpoint, accessToken)
			if err != nil {
				t.Errorf("failed to query media list:: %s", err.Error())
			}

			// assert
			expected := getValidMediaItem()
			is.Equal(response, expected)
		})
	}

}

func TestMediaQueryList(t *testing.T) {

	var tests = []struct {
		name      string
		mediaList micropub.MediaQueryListResponse
	}{
		{name: "it works without paging", mediaList: getValidMediaList()},
		{name: "it works with paging", mediaList: getValidMediaListWithPaging()},
	}

	for _, tt := range tests {

		is := is.NewRelaxed(t)
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			// arrange
			accessToken := "test-token"
			afterKey := ""
			mediaServer := newMediaServer(t, tt.mediaList)
			mediaEndpoint := mediaServer.URL
			logger := logrus.New()
			mpclient := micropub.NewClient(logger)

			// act
			response, err := mpclient.QueryMediaList(mediaEndpoint, accessToken, afterKey)
			if err != nil {
				t.Errorf("failed to query media list:: %s", err.Error())
			}

			// assert
			expected := tt.mediaList
			is.Equal(response, expected)
		})
	}

}

func getValidMediaList() micropub.MediaQueryListResponse {
	return micropub.MediaQueryListResponse{
		Items: []micropub.MediaQueryListResponseItem{
			micropub.MediaQueryListResponseItem{
				URL: "http://example.com/1",
			},
			micropub.MediaQueryListResponseItem{
				URL: "http://example.com/2",
			},
		}}
}
func getValidMediaListWithPaging() micropub.MediaQueryListResponse {
	paging := micropub.ListPaging{
		After: "123",
	}
	return micropub.MediaQueryListResponse{
		Items: []micropub.MediaQueryListResponseItem{
			micropub.MediaQueryListResponseItem{
				URL: "http://example.com/1",
			},
			micropub.MediaQueryListResponseItem{
				URL: "http://example.com/2",
			},
		},
		Paging: &paging,
	}
}
func getValidMediaItem() micropub.MediaQueryListResponseItem {
	return micropub.MediaQueryListResponseItem{
		URL: "http://example.com/1",
	}
}

func newMediaServer(t *testing.T, response micropub.MediaQueryListResponse) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {

				response, err := json.Marshal(response)
				if err != nil {
					t.Errorf("Failed to marshall json:: %s", err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err = w.Write(response)
				if err != nil {
					t.Errorf("Failed to write response body:: %s", err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			},
		),
	)
}

func newMediaServerSingleItem(t *testing.T, response micropub.MediaQueryListResponseItem) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {

				response, err := json.Marshal(response)
				if err != nil {
					t.Errorf("Failed to marshall json:: %s", err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err = w.Write(response)
				if err != nil {
					t.Errorf("Failed to write response body:: %s", err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			},
		),
	)
}
