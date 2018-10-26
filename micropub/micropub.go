package micropub

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/j4y_funabashi/inari-admin/storage"
	"github.com/sirupsen/logrus"
)

type MPClient interface {
	UploadToMediaServer(uploadedFile UploadedFile, usess storage.UserSession) (MediaEndpointResponse, error)
	SendRequest(body url.Values, endpoint, bearerToken string) (MicropubEndpointResponse, error)
}

func NewServer(logger *logrus.Logger, ss storage.SessionStore, client MPClient) server {
	s := server{
		logger:       logger,
		SessionStore: ss,
		client:       client,
	}
	return s
}

type server struct {
	logger       *logrus.Logger
	SessionStore storage.SessionStore
	client       MPClient
}

type HttpResponse struct {
	Headers    map[string]string
	Body       string
	StatusCode int
}

func (s *server) Routes(router *mux.Router) {
	router.HandleFunc("/composer", s.HandleComposerForm())
	router.HandleFunc("/composer/addphoto", s.HandleAddPhotoForm())
	router.HandleFunc("/submit", s.HandleSubmit())
}

func (s *server) HandleSubmit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		cookie, err := r.Cookie("sessionid")
		if err != nil {
			s.logger.Infof("redirecting, could not find sessionid cookie")
			w.Header().Set("Location", "/login")
			w.WriteHeader(http.StatusSeeOther)
			return
		}

		response := s.SubmitPost(
			cookie.Value,
			r.FormValue("content"),
			r.FormValue("h"),
		)
		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(response.StatusCode)
		w.Write([]byte(response.Body))
	}
}

func (s *server) SubmitPost(sessionid, content, h string) HttpResponse {

	// fetch session
	usess, err := s.SessionStore.FetchByID(sessionid)
	if err != nil {
		return HttpResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}
	}
	s.logger.WithFields(logrus.Fields{"user": usess}).Info("logged in user")

	// build POST body
	formData := url.Values{}
	formData.Add("content", content)
	formData.Add("h", h)
	for _, photo := range usess.ComposerData.Photos {
		formData.Add("photo", photo.URL)
	}

	published := time.Now().Format(time.RFC3339)
	if usess.ComposerData.Published != "" {
		published = usess.ComposerData.Published
	}
	formData.Add("published", published)
	if usess.ComposerData.Location != "" {
		formData.Add("location", usess.ComposerData.Location)
	}

	s.logger.WithFields(logrus.Fields{"request": formData}).Info("built micropub request")

	s.client.SendRequest(formData, usess.MicropubEndpoint, usess.AccessToken)

	// TODO redirect with message if not successful
	// TODO only clear session if successful
	usess.ClearComposerData()
	s.SessionStore.Create(usess)

	// redirect
	headers := map[string]string{
		"Location": "/composer",
	}
	return HttpResponse{
		StatusCode: http.StatusSeeOther,
		Headers:    headers,
	}
}

func (s *server) HandleComposerForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		cookie, err := r.Cookie("sessionid")
		if err != nil {
			s.logger.Infof("redirecting, could not find sessionid cookie")
			w.Header().Set("Location", "/login")
			w.WriteHeader(http.StatusSeeOther)
			return
		}

		response := s.ShowComposerForm(cookie.Value)
		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(response.StatusCode)
		w.Write([]byte(response.Body))
	}
}

func (s *server) HandleAddPhotoForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		cookie, err := r.Cookie("sessionid")
		if err != nil {
			s.logger.Infof("redirecting, could not find sessionid cookie")
			w.Header().Set("Location", "/login")
			w.WriteHeader(http.StatusSeeOther)
			return
		}

		response := HttpResponse{}

		switch r.Method {
		case "GET":
			response = s.ShowAddPhotoForm(cookie.Value)
		case "POST":
			err := r.ParseMultipartForm(32 << 20)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			photoFiles := []UploadedFile{}
			for _, photoFile := range r.MultipartForm.File["photo"] {
				file, err := photoFile.Open()
				if err != nil {
					s.logger.WithError(err).Error("failed to open file")
					continue
				}
				photoFiles = append(photoFiles, UploadedFile{Filename: photoFile.Filename, File: file})
			}
			response = s.AddPhotos(cookie.Value, photoFiles)
		}

		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(response.StatusCode)
		w.Write([]byte(response.Body))
	}
}

func (s *server) ShowComposerForm(sessionid string) HttpResponse {

	// fetch session
	usess, err := s.SessionStore.FetchByID(sessionid)
	if err != nil {
		return HttpResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}
	}
	s.logger.WithFields(logrus.Fields{"user": usess}).Info("logged in user")

	// render
	t, err := template.ParseFiles(
		"view/components.html",
		"view/layout.html",
		"view/composer.html",
	)
	if err != nil {
		return HttpResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}
	}

	w := new(bytes.Buffer)
	v := struct {
		PageTitle string
		Photos    []storage.MediaUpload
		User      storage.HCard
		Published string
		Location  string
	}{
		PageTitle: "Create Post",
		Photos:    usess.ComposerData.Photos,
		User:      usess.HCard,
		Published: usess.ComposerData.Published,
		Location:  usess.ComposerData.Location,
	}
	t.ExecuteTemplate(w, "layout", v)

	headers := map[string]string{
		"Content-Type": "text/html; charset=UTF-8",
	}

	return HttpResponse{
		StatusCode: http.StatusOK,
		Body:       w.String(),
		Headers:    headers,
	}
}

type UploadedFile struct {
	Filename string
	File     io.Reader
}

type MediaEndpointResponse struct {
	URL       string `json:"url"`
	Location  string `json:"location"`
	Published string `json:"published"`
}

func (s *server) AddPhotos(sessionid string, fileList []UploadedFile) HttpResponse {

	// checkSession
	usess, err := s.SessionStore.FetchByID(sessionid)
	if err != nil {
		return HttpResponse{
			StatusCode: http.StatusInternalServerError,
		}
	}
	s.logger.WithField("user", usess).
		Info("logged in user")

	// upload photos to media endpoint
	s.logger.WithField("media_endpoint", usess.MediaEndpoint).
		Info("sending photos to media endpoint")

	for _, photoFile := range fileList {
		res, err := s.client.UploadToMediaServer(photoFile, usess)
		if err != nil {
			s.logger.WithError(err).Error("failed to upload to media endpoint")
			continue
		}
		s.logger.
			WithField("media_endpoint_response", res).
			Info("media endpoint response")

		// add uploaded photos + errors to session
		usess.AddPhotoUpload(res.URL, res.Published, res.Location)
		err = s.SessionStore.Create(usess)
		if err != nil {
			s.logger.WithError(err).Error("failed to save session")
			continue
		}

		s.logger.
			WithField("session", usess).
			Info("user session")
	}

	// redirect
	headers := map[string]string{
		"Location": "/composer",
	}
	return HttpResponse{
		StatusCode: http.StatusSeeOther,
		Headers:    headers,
	}
}

// Client provides methods to send requests to a micropub server and
// handle the responses
type Client struct {
	logger *logrus.Logger
}

func NewClient(logger *logrus.Logger) Client {
	return Client{
		logger: logger,
	}
}

func (mpclient Client) UploadToMediaServer(uploadedFile UploadedFile, usess storage.UserSession) (MediaEndpointResponse, error) {
	// copy file to multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", uploadedFile.Filename)
	if err != nil {
		mpclient.logger.WithError(err).Error("failed to create multipart")
		return MediaEndpointResponse{}, err
	}

	_, err = io.Copy(part, uploadedFile.File)
	if err != nil {
		mpclient.logger.WithError(err).Error("failed to copy file into multipart")
		return MediaEndpointResponse{}, err
	}

	err = writer.Close()
	if err != nil {
		mpclient.logger.WithError(err).Error("failed to close multipart writer")
		return MediaEndpointResponse{}, err
	}

	// create media-endpoint request
	req, err := http.NewRequest("POST", usess.MediaEndpoint, body)
	if err != nil {
		mpclient.logger.WithError(err).Error("failed to create request")
		return MediaEndpointResponse{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+usess.AccessToken)

	// perform request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		mpclient.logger.WithError(err).Error("failed to perform request")
		return MediaEndpointResponse{}, err
	}

	// read media-endpoint response
	respBody := &bytes.Buffer{}
	_, err = respBody.ReadFrom(resp.Body)
	if err != nil {
		mpclient.logger.WithError(err).Error("failed to read response body")
		return MediaEndpointResponse{}, err
	}
	mpclient.logger.
		WithField("response_body", respBody.String()).
		Info("media uploaded")

	mediaResponse := MediaEndpointResponse{}
	err = json.Unmarshal(respBody.Bytes(), &mediaResponse)
	if err != nil {
		mpclient.logger.
			WithError(err).
			WithField("response_body", respBody.String()).
			Error("failed to umarshal media endpoint response")
	}

	mediaResponse.URL = resp.Header.Get("location")

	return mediaResponse, nil
}

type MicropubEndpointResponse struct {
	StatusCode int
	Location   string
}

func (mpclient Client) SendRequest(body url.Values, mpEndpoint, bearerToken string) (MicropubEndpointResponse, error) {

	req, err := http.NewRequest("POST", mpEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		mpclient.logger.WithError(err).Error("failed to create request")
		return MicropubEndpointResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// perform request
	mpclient.logger.WithField("micropub_endpoint", mpEndpoint).Info("sending micropub request")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		mpclient.logger.WithError(err).Error("failed to perform request")
		return MicropubEndpointResponse{}, err
	}
	mpclient.logger.WithField("micropub_response", resp.StatusCode).Info("micropub response")

	return MicropubEndpointResponse{
		StatusCode: resp.StatusCode,
		Location:   resp.Header.Get("location"),
	}, nil
}

func (s *server) ShowAddPhotoForm(sessionid string) HttpResponse {

	// checkSession
	usess, err := s.SessionStore.FetchByID(sessionid)
	if err != nil {
		return HttpResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}
	}
	s.logger.WithFields(logrus.Fields{"user": usess}).Info("logged in user")

	// render
	t, err := template.ParseFiles(
		"view/components.html",
		"view/layout.html",
		"view/newphoto.html",
	)
	if err != nil {
		return HttpResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}
	}

	w := new(bytes.Buffer)
	v := struct {
		PageTitle string
	}{
		PageTitle: "Add Photo",
	}
	t.ExecuteTemplate(w, "layout", v)

	headers := map[string]string{
		"Content-Type": "text/html; charset=UTF-8",
	}

	return HttpResponse{
		StatusCode: http.StatusOK,
		Body:       w.String(),
		Headers:    headers,
	}
}
