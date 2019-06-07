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
	HumanDateLayout   = "Mon, Jan 02, 2006 15:04 -0700"
	MachineDateLayout = time.RFC3339
)

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

type Month struct {
	Month string
	Count int
	Link  string
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
}

type Media struct {
	URL          string
	BorderColour string
}

func parseMediaList(media []okami.Media) []Media {
	out := []Media{}

	for _, m := range media {

		bc := "near-white"
		if m.IsPublished {
			bc = "red"
		}
		m := Media{
			URL:          m.URL,
			BorderColour: bc,
		}
		out = append(out, m)
	}

	return out
}

func ParseListMediaView(mediaResponse okami.ListMediaResponse) ListMediaView {
	months := parseMonths(mediaResponse.Months, mediaResponse.CurrentYear)
	years := parseYears(mediaResponse.Years)
	cm := parseMonth(mediaResponse.CurrentMonth)
	cy := mediaResponse.CurrentYear
	media := parseMediaList(mediaResponse.Media)
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
	}
}

func RenderMediaList(mediaResponse okami.ListMediaResponse, outBuf *bytes.Buffer) error {

	viewModel := ParseListMediaView(mediaResponse)

	t, err := template.ParseFiles(
		"view/components.html",
		"view/layout.html",
		"view/medialist.html",
	)
	if err != nil {
		return err
	}
	err = t.ExecuteTemplate(outBuf, "layout", viewModel)
	return err
}
