package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/j4y_funabashi/inari-admin/login"
)

func Handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	logger := NewLogger()
	loginServer := login.NewServer(
		logger,
		authClient,
		clientID,
		redirectURL,
	)

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
