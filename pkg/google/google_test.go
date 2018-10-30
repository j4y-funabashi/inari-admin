package google_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/j4y_funabashi/inari-admin/pkg/google"
	"github.com/j4y_funabashi/inari-admin/pkg/session"
	"github.com/matryer/is"
	log "github.com/sirupsen/logrus"
)

func TestLookup(t *testing.T) {
	is := is.NewRelaxed(t)

	var tests = []struct {
		name                string
		address             string
		apiKey              string
		fixtureFile         string
		expected            []session.Location
		expectedQueryString string
	}{
		{
			name:    "happy path",
			address: "123 test street",
			apiKey:  "123apiKey",
			expected: []session.Location{
				{
					Lat:      53.8324973,
					Lng:      -1.5698563,
					Locality: "Meanwood",
					Region:   "West Yorkshire",
					Country:  "United Kingdom",
				},
			},
			expectedQueryString: "address=123+test+street&key=123apiKey",
			fixtureFile:         "testdata/result.json",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			logger := log.New()

			geoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// assert url is correct
				is.Equal(tt.expectedQueryString, r.URL.RawQuery)
				// load fixtures
				data, err := ioutil.ReadFile(tt.fixtureFile)
				if err != nil {
					t.Fatalf("failed to open test fixture: %s", err.Error())
				}
				buf := bytes.NewBuffer(data)
				io.Copy(w, buf)
				w.WriteHeader(200)
			}))

			geocoder := google.NewGeocoder(tt.apiKey, geoServer.URL, logger)

			// act
			result := geocoder.Lookup(tt.address)

			// assert
			is.Equal(tt.expected, result)
		})
	}
}
