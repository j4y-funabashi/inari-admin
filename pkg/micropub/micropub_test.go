package micropub_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/j4y_funabashi/inari-admin/pkg/micropub"
	"github.com/sirupsen/logrus"
)

func TestClientMediaQueryList(t *testing.T) {

	var tests = []struct {
		name string
	}{
		{"it works"},
	}

	//is := is.NewRelaxed(t)

	for _, tt := range tests {

		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			// arrange
			accessToken := "test-token"
			mediaServer := newMediaServer()
			mediaEndpoint := mediaServer.URL
			logger := logrus.New()
			mpclient := micropub.NewClient(logger)

			// act
			response, err := mpclient.QueryMediaList(mediaEndpoint, accessToken)
			if err != nil {
				t.Errorf("ERRORZ:: %+v", err)
			}
			t.Errorf("RESPONSE:: %+v", response)

			// assert
		})
	}

}

func newMediaServer() *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
			},
		),
	)
}
