package main

import (
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/j4y_funabashi/inari-admin/indieauth"
	"github.com/j4y_funabashi/inari-admin/login"
	"github.com/j4y_funabashi/inari-admin/micropub"
	"github.com/j4y_funabashi/inari-admin/storage"
)

func main() {

	// config
	port := "8089"
	region := "eu-central-1"
	bucket := "admin.funabashi.co.uk"
	clientID := "https://admin.funabashi.co.uk"
	redirectURL := "http://localhost:" + port + "/login-callback"

	// deps
	logger := log.New()
	logger.Formatter = &log.JSONFormatter{}
	router := mux.NewRouter()

	sstore, err := storage.NewS3SessionStore(region, bucket)
	if err != nil {
		logger.WithError(err).Fatal("failed to create session store")
	}
	authClient := indieauth.NewClient("", sstore, logger)

	// servers
	loginServer := login.NewServer(
		logger,
		authClient,
		clientID,
		redirectURL,
	)
	loginServer.Routes(router)

	micropubClientServer := micropub.NewServer(logger)
	micropubClientServer.Routes(router)

	logger.Info("server running on port " + port)
	logger.Fatal(http.ListenAndServe(":"+port, router))
}
