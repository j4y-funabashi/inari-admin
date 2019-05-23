package view

import (
	"bytes"
	"text/template"
	"time"
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
