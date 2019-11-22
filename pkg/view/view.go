package view

import (
	"bytes"
	"fmt"
	"net/url"
	"text/template"
	"time"

	"github.com/j4y_funabashi/inari-admin/pkg/okami"
)

const (
	HumanDateLayout   = "Mon, Jan 02, 2006 15:04"
	HumanDayLayout    = "Mon, 02 January"
	MachineDateLayout = time.RFC3339
)

type Month struct {
	Month string
	Count int
	Link  string
}

type Year struct {
	Year  string
	Count int
	Link  string
}

type ListMediaView struct {
	Months       []Month
	Years        []Year
	CurrentMonth string
	CurrentYear  string
	Media        []Media
	AfterKey     string
	HasPaging    bool
	PageTitle    string
	MediaDays    []MediaDay
}

type MediaDay struct {
	Date           string
	Media          []Media
	Count          int
	PublishedCount int
	Link           string
}

type Media struct {
	URL         string
	HumanDate   string
	IsPublished bool
	MachineDate string
	Lat         float64
	Lng         float64
}

type MediaItem struct {
	URL      string     `json:"url"`
	MimeType string     `json:"mime_type"`
	DateTime *time.Time `json:"date_time"`
	Lat      float64    `json:"lat"`
	Lng      float64    `json:"lng"`
}

func (media MediaItem) HumanDate() string {
	return media.DateTime.Format(HumanDateLayout)
}

func (media MediaItem) MachineDate() string {
	return media.DateTime.Format(MachineDateLayout)
}

func (media MediaItem) HasLocation() bool {
	return media.Lat > 0 || media.Lng > 0
}

func RenderMediaPreview(media MediaItem, outBuf *bytes.Buffer) error {

	t, err := template.ParseFiles(
		"view/components.html",
		"view/layout.html",
		"view/mediapreview.html",
	)
	if err != nil {
		return err
	}
	v := struct {
		PageTitle string
		Media     MediaItem
	}{
		PageTitle: "Choose a Video/Photo",
		Media:     media,
	}
	err = t.ExecuteTemplate(outBuf, "layout", v)
	return err
}

func parseMonth(month string) string {
	dat, _ := time.Parse("1", fmt.Sprintf("%s", month))
	return dat.Format("January")
}

func parseMonths(months []okami.ArchiveMonth, year string) []Month {
	out := []Month{}

	for _, month := range months {

		humanMonth := parseMonth(month.Month)
		dat := fmt.Sprintf("%s, %s", humanMonth, year)

		link := url.Values{}
		link.Add("year", year)
		link.Add("month", month.Month)

		m := Month{
			Month: dat,
			Count: month.Count,
			Link:  fmt.Sprintf("?%s", link.Encode()),
		}
		out = append(out, m)
	}

	return out
}

func parseYears(years []okami.ArchiveYear) []Year {
	out := []Year{}

	for _, y := range years {

		link := url.Values{}
		link.Add("year", y.Year)

		m := Year{
			Year:  y.Year,
			Count: y.Count,
			Link:  fmt.Sprintf("?%s", link.Encode()),
		}
		out = append(out, m)
	}

	return out
}

func parseMediaDays(media []okami.Media, currentMonth, currentYear string) []MediaDay {
	out := []MediaDay{}

	dayMap := make(map[string][]Media)
	dayList := []string{}
	for _, m := range media {
		_, exists := dayMap[m.DateTime.Format("2006-01-02")]
		dayMap[m.DateTime.Format("2006-01-02")] = append(
			dayMap[m.DateTime.Format("2006-01-02")],
			parseMedia(m),
		)
		if !exists {
			dayList = append(dayList, m.DateTime.Format("2006-01-02"))
		}
	}

	for _, day := range dayList {
		mediaDay, _ := time.Parse("2006-01-02", day)
		dayLink := fmt.Sprintf("?month=%s&year=%s&day=%d", currentMonth, currentYear, mediaDay.Day())
		limit := 3
		if limit > len(dayMap[day]) {
			limit = len(dayMap[day])
		}
		out = append(
			out,
			MediaDay{
				Date:           mediaDay.Format(HumanDayLayout),
				Media:          dayMap[day][0:limit],
				Count:          len(dayMap[day]),
				PublishedCount: countPublished(dayMap[day]),
				Link:           dayLink,
			},
		)
	}

	return out
}

func countPublished(media []Media) int {
	count := 0

	for _, m := range media {
		if m.IsPublished {
			count++
		}
	}

	return count
}

func parseMedia(media okami.Media) Media {
	url := media.URL
	m := Media{
		URL:         url,
		IsPublished: media.IsPublished,
		HumanDate:   media.DateTime.Format(HumanDateLayout),
		MachineDate: media.DateTime.Format(MachineDateLayout),
		Lat:         media.Lat,
		Lng:         media.Lng,
	}
	return m
}

func parseMediaList(media []okami.Media) []Media {
	out := []Media{}

	for _, m := range media {
		out = append(out, parseMedia(m))
	}

	return out
}

func parseMediaGrid(media []okami.Media) [][]Media {
	columnCount := 4
	out := [][]Media{}

	i := 1
	column := []Media{}
	for _, m := range media {
		column = append(column, parseMedia(m))
		if i%columnCount == 0 {
			out = append(out, column)
			column = []Media{}
		}
		i++
	}
	if len(column) > 0 {
		out = append(out, column)
	}

	return out
}

func filterMediaDay(media []okami.Media, selectedDay string) []okami.Media {

	out := []okami.Media{}

	for _, m := range media {
		if m.DateTime.Format("2") == selectedDay {
			out = append(out, m)
		}
	}

	return out
}

func ParseListMediaView(mediaResponse okami.ListMediaResponse) ListMediaView {
	months := parseMonths(mediaResponse.Months, mediaResponse.CurrentYear)
	years := parseYears(mediaResponse.Years)
	cm := parseMonth(mediaResponse.CurrentMonth)
	cy := mediaResponse.CurrentYear
	media := parseMediaList(mediaResponse.Media)
	mediaDays := parseMediaDays(mediaResponse.Media, mediaResponse.CurrentMonth, mediaResponse.CurrentYear)
	ak := mediaResponse.AfterKey

	return ListMediaView{
		Months:       months,
		Years:        years,
		CurrentMonth: cm,
		CurrentYear:  cy,
		Media:        media,
		AfterKey:     ak,
		HasPaging:    ak != "",
		PageTitle:    "Choose some shiz to shizzle with",
		MediaDays:    mediaDays,
	}
}

type MediaDayView struct {
	Months       []Month
	Years        []Year
	CurrentMonth string
	CurrentYear  string
	Media        []Media
	MediaGrid    [][]Media
	PageTitle    string
}

func ParseMediaDayView(mediaResponse okami.ListMediaResponse, selectedDay string) MediaDayView {
	months := parseMonths(mediaResponse.Months, mediaResponse.CurrentYear)
	years := parseYears(mediaResponse.Years)
	cm := parseMonth(mediaResponse.CurrentMonth)
	cy := mediaResponse.CurrentYear
	media := parseMediaList(filterMediaDay(mediaResponse.Media, selectedDay))
	mediaGrid := parseMediaGrid(filterMediaDay(mediaResponse.Media, selectedDay))

	return MediaDayView{
		Months:       months,
		Years:        years,
		CurrentMonth: cm,
		CurrentYear:  cy,
		Media:        media,
		MediaGrid:    mediaGrid,
		PageTitle:    "Choose some shiz to shizzle with",
	}
}

func RenderMediaDay(mediaResponse okami.ListMediaResponse, selectedDay string, outBuf *bytes.Buffer) error {

	viewModel := ParseMediaDayView(mediaResponse, selectedDay)

	t, err := template.ParseFiles(
		"view/components.html",
		"view/layout.html",
		"view/mediaday.html",
		"view/media-thumbnail.html",
	)
	if err != nil {
		return err
	}
	err = t.ExecuteTemplate(outBuf, "layout", viewModel)
	return err
}

func RenderMediaList(mediaResponse okami.ListMediaResponse, outBuf *bytes.Buffer) error {

	viewModel := ParseListMediaView(mediaResponse)

	t, err := template.ParseFiles(
		"view/components.html",
		"view/layout.html",
		"view/medialist.html",
		"view/media-thumbnail.html",
	)
	if err != nil {
		return err
	}
	err = t.ExecuteTemplate(outBuf, "layout", viewModel)
	return err
}
