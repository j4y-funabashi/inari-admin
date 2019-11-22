package main

import (
	"flag"

	"github.com/j4y_funabashi/inari-admin/pkg/micropub"
	log "github.com/sirupsen/logrus"
)

func main() {
	// config
	mpEndpoint := "https://jay.funabashi.co.uk/micropub"
	accessToken := flag.String("token", "", "micropub access token")
	flag.Parse()

	// deps
	logger := log.New()
	mpClient := micropub.NewClient(logger)

	logger.Info("Hello!")

	// FETCH DATA
	err := fetchPostList(mpClient, mpEndpoint, accessToken, "")
	if err != nil {
		logger.WithError(err).Error("Failed to fetch post list")
		return
	}

	// SAVE DATA
	logger.
		Info("complete")
}

func fetchPostList(
	mpClient micropub.Client,
	mpEndpoint,
	accessToken,
	afterKey string,
) error {

	postList, err := mpClient.QueryPostList(mpEndpoint, accessToken, afterKey)
	if err != nil {
		return err
	}

	if postList.Paging != nil && postList.Paging.After != "" {
		err := fetchPostList(mpClient, mpEndpoint, accessToken, postList.Paging.After)
		if err != nil {
			return err
		}
	}
	return err
}
