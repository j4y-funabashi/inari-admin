package okami

import (
	"time"

	"github.com/j4y_funabashi/inari-admin/pkg/mf2"
	"github.com/j4y_funabashi/inari-admin/pkg/mpclient"
	"github.com/sirupsen/logrus"
)

type Server struct {
	mpClient MPClient
	logger   *logrus.Logger
}

type ListMediaResponse struct {
	Years    []ArchiveYear
	Media    []Media
	AfterKey string
}
type ArchiveYear struct {
	Year  string
	Count int
}
type Media struct {
	URL         string     `json:"url"`
	MimeType    string     `json:"mime_type"`
	DateTime    *time.Time `json:"date_time"`
	Lat         float64    `json:"lat"`
	Lng         float64    `json:"lng"`
	IsPublished bool       `json:"is_published"`
}

type MPClient interface {
	QueryYearsList(micropubEndpoint, accessToken string) ([]mf2.ArchiveYear, error)
	QueryMediaList(mediaEndpoint, accessToken, afterKey string) (mpclient.MediaQueryListResponse, error)
}

func New(mpClient MPClient, logger *logrus.Logger) Server {
	return Server{
		mpClient: mpClient,
		logger:   logger,
	}
}

func (s Server) ListMedia(mediaEndpoint, accessToken, afterKey string) ListMediaResponse {

	media, newAfterKey := s.listMedia(mediaEndpoint, accessToken, afterKey)
	years := s.listMediaYears(mediaEndpoint, accessToken)

	return ListMediaResponse{
		Years:    years,
		Media:    media,
		AfterKey: newAfterKey,
	}
}

func (s Server) listMediaYears(mediaEndpoint, accessToken string) []ArchiveYear {
	var years []ArchiveYear
	yearsList, err := s.mpClient.QueryYearsList(
		mediaEndpoint,
		accessToken,
	)
	if err != nil {
		s.logger.WithError(err).
			Info("failed to query year list")
		return years
	}
	for _, y := range yearsList {
		years = append(years, ArchiveYear{Year: y.Year, Count: y.Count})
	}

	return years
}

func (s Server) listMedia(mediaEndpoint, accessToken, afterKey string) ([]Media, string) {
	var media []Media
	mediaList, err := s.mpClient.QueryMediaList(
		mediaEndpoint,
		accessToken,
		afterKey,
	)
	if err != nil {
		s.logger.WithError(err).
			Info("failed to query media list")
		return media, ""
	}
	for _, mediaItem := range mediaList.Items {
		media = append(
			media,
			Media{
				URL:         mediaItem.URL,
				IsPublished: mediaItem.IsPublished,
			},
		)
	}

	newAfterKey := ""
	if mediaList.Paging != nil {
		newAfterKey = mediaList.Paging.After
	}

	return media, newAfterKey
}
