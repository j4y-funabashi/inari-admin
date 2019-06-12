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
	Years        []ArchiveYear
	Months       []ArchiveMonth
	Media        []Media
	AfterKey     string
	CurrentYear  string
	CurrentMonth string
}
type ArchiveYear struct {
	Year  string
	Count int
}
type ArchiveMonth struct {
	Month string
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
	QueryMonthsList(micropubEndpoint, accessToken, currentYear string) ([]mf2.ArchiveMonth, error)
	QueryMediaList(mediaEndpoint, accessToken, afterKey, year, month string) (mpclient.MediaQueryListResponse, error)
}

func New(mpClient MPClient, logger *logrus.Logger) Server {
	return Server{
		mpClient: mpClient,
		logger:   logger,
	}
}

func (s Server) ListMedia(mediaEndpoint, accessToken, afterKey, selectedYear, selectedMonth string) ListMediaResponse {

	years := s.listMediaYears(mediaEndpoint, accessToken)
	currentYear := selectedYear
	if currentYear == "" {
		currentYear = years[0].Year
	}

	months := s.listMediaMonths(mediaEndpoint, accessToken, currentYear)
	currentMonth := selectedMonth
	if currentMonth == "" {
		currentMonth = months[0].Month
	}

	media, newAfterKey := s.listMedia(
		mediaEndpoint,
		accessToken,
		afterKey,
		currentYear,
		currentMonth,
	)

	return ListMediaResponse{
		Years:        years,
		Media:        media,
		Months:       months,
		AfterKey:     newAfterKey,
		CurrentYear:  currentYear,
		CurrentMonth: currentMonth,
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

func (s Server) listMediaMonths(mediaEndpoint, accessToken, currentYear string) []ArchiveMonth {
	var months []ArchiveMonth
	yearsList, err := s.mpClient.QueryMonthsList(
		mediaEndpoint,
		accessToken,
		currentYear,
	)
	if err != nil {
		s.logger.WithError(err).
			Info("failed to query year list")
		return months
	}
	for _, y := range yearsList {
		months = append(months, ArchiveMonth{Month: y.Month, Count: y.Count})
	}

	return months
}

func (s Server) listMedia(mediaEndpoint, accessToken, afterKey, year, month string) ([]Media, string) {
	var media []Media
	mediaList, err := s.mpClient.QueryMediaList(
		mediaEndpoint,
		accessToken,
		afterKey,
		year,
		month,
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
				DateTime:    mediaItem.DateTime,
			},
		)
	}

	newAfterKey := ""
	if mediaList.Paging != nil {
		newAfterKey = mediaList.Paging.After
	}

	return media, newAfterKey
}
