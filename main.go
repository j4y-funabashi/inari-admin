package main

import (
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/j4y_funabashi/inari-admin/pkg/google"
	"github.com/j4y_funabashi/inari-admin/pkg/indieauth"
	"github.com/j4y_funabashi/inari-admin/pkg/login"
	"github.com/j4y_funabashi/inari-admin/pkg/micropub"
	"github.com/j4y_funabashi/inari-admin/pkg/okami"
	"github.com/j4y_funabashi/inari-admin/pkg/session"
	log "github.com/sirupsen/logrus"
)

func main() {

	// config
	port := "80"
	sessionBucketRegion := "eu-central-1"
	sessionBucket := os.Getenv("SESSION_BUCKET")
	clientID := os.Getenv("CLIENT_ID")
	redirectURL := os.Getenv("CALLBACK_URL")
	geoAPIKey := os.Getenv("GEO_API_KEY")
	geoBaseURL := os.Getenv("GEO_BASE_URL")

	// deps
	logger := log.New()
	logger.Formatter = &log.JSONFormatter{}

	sstore, err := session.NewS3SessionStore(sessionBucketRegion, sessionBucket)
	if err != nil {
		logger.WithError(err).Fatal("failed to create session store")
	}
	authClient := indieauth.NewClient("", sstore, logger)
	mpClient := micropub.NewClient(logger)

	geoCoder := google.NewGeocoder(geoAPIKey, geoBaseURL, logger)

	// routes
	router := mux.NewRouter()
	router.Use(newLoggerMiddleware(logger))

	// servers
	loginServer := login.NewServer(
		logger,
		authClient,
		clientID,
		redirectURL,
	)
	loginServer.Routes(router)

	app := okami.New(mpClient, logger)

	micropubClientServer := micropub.NewServer(
		logger,
		sstore,
		mpClient,
		geoCoder,
		app,
	)
	micropubClientServer.Routes(router)

	logger.Info("server running on port " + port)

	logger.Fatal(http.ListenAndServe(
		":"+port,
		router,
	))
}

func newLoggerMiddleware(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.
				WithField("path", r.RequestURI).
				WithField("method", r.Method).
				Debug("received request")
			next.ServeHTTP(w, r)
		})
	}
}
