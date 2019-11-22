package google_test

import (
	"testing"

	"github.com/j4y_funabashi/inari-admin/pkg/google"
	log "github.com/sirupsen/logrus"
)

func TestLookup(t *testing.T) {

	apiKey := ""
	baseURL := "https://maps.googleapis.com/maps/api/geocode/json"
	address := "oprtalj"

	// deps
	logger := log.New()
	logger.Formatter = &log.JSONFormatter{}

	sut := google.NewGeocoder(apiKey, baseURL, logger)

	result := sut.Lookup(address)

	t.Errorf("%+v", result)
}

func TestLookupLatLng(t *testing.T) {

	apiKey := ""
	baseURL := "https://maps.googleapis.com/maps/api/geocode/json"
	lat := 53.80097961111111
	lng := -1.5413867222222222

	// deps
	logger := log.New()
	logger.Formatter = &log.JSONFormatter{}

	sut := google.NewGeocoder(apiKey, baseURL, logger)

	result := sut.LookupLatLng(lat, lng)

	t.Errorf("%+v", result)
}
