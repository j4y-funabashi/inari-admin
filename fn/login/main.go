package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/j4y_funabashi/inari-admin/indieauth"
	"github.com/j4y_funabashi/inari-admin/login"
	"github.com/j4y_funabashi/inari-admin/storage"
)

func Handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	// config
	region := "eu-central-1"
	bucket := "admin.funabashi.co.uk"
	clientID := "https://admin.funabashi.co.uk"
	redirectURL := "https://admin.funabashi.co.uk/login-callback"

	// deps
	logger := NewLogger()
	sstore, err := storage.NewS3SessionStore(region, bucket)
	if err != nil {
		logger.WithError(err).Error("failed to create session store")
		return events.APIGatewayProxyResponse{}, err
	}
	authClient := indieauth.NewClient("", sstore, logger)

	loginServer := login.NewServer(
		logger,
		authClient,
		clientID,
		redirectURL,
	)

	response := loginServer.ShowLoginForm()

	return events.APIGatewayProxyResponse{
		Body:       response.Body,
		Headers:    response.Headers,
		StatusCode: response.StatusCode,
	}, nil
}

func NewLogger() *log.Logger {
	l := log.New()
	l.Formatter = &log.JSONFormatter{}
	return l
}

func main() {
	lambda.Start(Handler)
}
