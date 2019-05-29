package mpclient

import "time"

type MediaQueryListResponse struct {
	Items  []MediaQueryListResponseItem `json:"items"`
	Paging *ListPaging                  `json:"paging,omitempty"`
}
type MediaQueryListResponseItem struct {
	URL         string     `json:"url"`
	MimeType    string     `json:"mime_type"`
	DateTime    *time.Time `json:"date_time"`
	Lat         float64    `json:"lat"`
	Lng         float64    `json:"lng"`
	IsPublished bool       `json:"is_published"`
}
type ListPaging struct {
	After string `json:"after"`
}
