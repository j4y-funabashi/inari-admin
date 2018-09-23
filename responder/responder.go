package responder

import (
	"bytes"
	"html/template"
)

type Response struct {
	Headers    map[string]string
	Body       string
	StatusCode int
}

func NewPhotoForm(IMAGE_PROXY, mediaEndpoint, accessToken string) (Response, error) {

	var response Response
	w := new(bytes.Buffer)
	t, err := template.ParseFiles(
		"view/components.html",
		"view/layout.html",
		"view/newphoto.html",
	)
	if err != nil {
		return response, err
	}
	err = t.ExecuteTemplate(
		w,
		"layout",
		struct {
			PageTitle     string
			ImageProxy    string
			MediaEndpoint string
			BearerToken   string
		}{
			PageTitle:     "New Photo",
			ImageProxy:    IMAGE_PROXY,
			MediaEndpoint: mediaEndpoint,
			BearerToken:   accessToken,
		},
	)
	if err != nil {
		return response, err
	}

	headers := map[string]string{
		"Content-Type": "text/html; charset=UTF-8",
	}

	response.StatusCode = 200
	response.Body = w.String()
	response.Headers = headers
	return response, nil
}
