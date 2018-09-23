package main

import (
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/j4y_funabashi/inari/responder"
	"github.com/j4y_funabashi/inari/storage"
)

func Handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	IMAGE_PROXY := os.Getenv("IMAGE_PROXY")

	sstore, err := storage.NewDynamoSessionStore()
	if err != nil {
		log.Printf("failed to connect to db: %v", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	cookie, err := url.ParseQuery(request.Headers["Cookie"])
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	sessionid := cookie["sessionid"][0]
	usess, err := sstore.FetchByID(sessionid)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	log.Printf("%+v", usess)

	response, err := responder.NewPhotoForm(IMAGE_PROXY, usess.MediaEndpoint, usess.AccessToken)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	return events.APIGatewayProxyResponse{
		Body:       response.Body,
		Headers:    response.Headers,
		StatusCode: response.StatusCode,
	}, nil
}

func main() {
	lambda.Start(Handler)
}
