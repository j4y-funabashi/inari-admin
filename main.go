package main

import (
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/j4y_funabashi/inari-admin/pkg/indieauth"
	"github.com/j4y_funabashi/inari-admin/pkg/login"
	"github.com/j4y_funabashi/inari-admin/pkg/micropub"
	"github.com/j4y_funabashi/inari-admin/pkg/session"
	log "github.com/sirupsen/logrus"
)

func main() {

	// config
	port := "80"
	region := "eu-central-1"
	bucket := "admin.funabashi.co.uk"
	clientID := "https://admin.funabashi.co.uk"
	redirectURL := os.Getenv("CALLBACK_URL")

	// deps
	logger := log.New()
	logger.Formatter = &log.JSONFormatter{}
	router := mux.NewRouter()

	sstore, err := session.NewS3SessionStore(region, bucket)
	if err != nil {
		logger.WithError(err).Fatal("failed to create session store")
	}
	authClient := indieauth.NewClient("", sstore, logger)
	mpClient := micropub.NewClient(logger)

	// servers
	loginServer := login.NewServer(
		logger,
		authClient,
		clientID,
		redirectURL,
	)
	loginServer.Routes(router)

	micropubClientServer := micropub.NewServer(logger, sstore, mpClient)
	micropubClientServer.Routes(router)

	logger.Info("server running on port " + port)
	logger.Fatal(http.ListenAndServe(":"+port, router))
}
