package micropub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/j4y_funabashi/inari-admin/pkg/mf2"
	"github.com/j4y_funabashi/inari-admin/pkg/mpclient"
	"github.com/j4y_funabashi/inari-admin/pkg/okami"
	"github.com/j4y_funabashi/inari-admin/pkg/session"
	"github.com/j4y_funabashi/inari-admin/pkg/view"
	"github.com/sirupsen/logrus"
)

type MPClient interface {
	UploadToMediaServer(uploadedFile UploadedFile, usess session.UserSession) (MediaEndpointResponse, error)
	SendRequest(body url.Values, endpoint, bearerToken string) (MicropubEndpointResponse, error)
	QueryPostList(micropubEndpoint, accessToken, afterKey string) (mf2.PostList, error)
	QueryYearsList(micropubEndpoint, accessToken string) ([]mf2.ArchiveYear, error)
	QueryMediaList(mediaEndpoint, accessToken, afterKey, year, month string) (mpclient.MediaQueryListResponse, error)
	QueryMediaURL(URL, mediaEndpoint, accessToken string) (mpclient.MediaQueryListResponseItem, error)
}

type GeoCoder interface {
	Lookup(address string) []session.Location
}

func NewServer(
	logger *logrus.Logger,
	ss session.SessionStore,
	client MPClient,
	geocoder GeoCoder,
	app okami.Server,
) server {
	s := server{
		logger:       logger,
		SessionStore: ss,
		client:       client,
		geocoder:     geocoder,
		app:          app,
	}
	return s
}

type server struct {
	logger       *logrus.Logger
	SessionStore session.SessionStore
	client       MPClient
	geocoder     GeoCoder
	app          okami.Server
}

type HttpResponse struct {
	Headers    map[string]string
	Body       string
	StatusCode int
}

func (s *server) Routes(router *mux.Router) {
	router.HandleFunc("/composer", s.HandleComposerForm())
	router.HandleFunc("/composer/addlocation", s.HandleAddLocationForm())
	router.HandleFunc("/submit", s.HandleSubmit())
	router.HandleFunc("/composer/media", s.HandleAddMediaToComposer()).Methods("POST")
	router.HandleFunc("/composer/media/device", s.HandleAddPhotoForm())
	router.HandleFunc("/composer/media/gallery", s.HandleQueryMedia())
	router.HandleFunc("/queryposts", s.HandleQueryPosts())
}

func (s *server) HandleQueryMedia() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// fetch cookie
		cookie, err := r.Cookie("sessionid")
		if err != nil {
			s.logger.WithError(err).Info("could not find sessionid cookie")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		// fetch session
		usess, err := s.SessionStore.FetchByID(cookie.Value)
		if err != nil {
			s.logger.WithError(err).Info("could not find session")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		s.logger.WithField("user", usess).Info("logged in user")

		mediaURL := r.URL.Query().Get("url")
		outBuf := new(bytes.Buffer)
		if len(mediaURL) > 0 {
			// fetch media
			mediaResponse, err := s.client.QueryMediaURL(
				mediaURL,
				usess.MediaEndpoint,
				usess.AccessToken,
			)
			if err != nil {
				s.logger.WithError(err).Info("failed to query media list")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// render media
			viewModel := view.MediaItem{
				URL:      mediaResponse.URL,
				MimeType: mediaResponse.MimeType,
				DateTime: mediaResponse.DateTime,
				Lat:      mediaResponse.Lat,
				Lng:      mediaResponse.Lng,
			}
			err = view.RenderMediaPreview(viewModel, outBuf)
			if err != nil {
				s.logger.WithError(err).Error("failed to parse template files")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

		} else {

			afterKey := r.URL.Query().Get("after")
			selectedMonth := r.URL.Query().Get("month")
			selectedYear := r.URL.Query().Get("year")
			selectedDay := r.URL.Query().Get("day")
			mediaResponse := s.app.ListMedia(
				usess.MediaEndpoint,
				usess.AccessToken,
				afterKey,
				selectedYear,
				selectedMonth,
			)

			if selectedDay == "" {
				err = view.RenderMediaList(mediaResponse, outBuf)
			} else {
				err = view.RenderMediaDay(mediaResponse, selectedDay, outBuf)
			}

			if err != nil {
				s.logger.WithError(err).Error("failed to parse template files")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		w.Write(outBuf.Bytes())
	}
}

func (s *server) HandleAddMediaToComposer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// fetch cookie
		cookie, err := r.Cookie("sessionid")
		if err != nil {
			s.logger.WithError(err).Info("could not find sessionid cookie")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		// fetch session
		usess, err := s.SessionStore.FetchByID(cookie.Value)
		if err != nil {
			s.logger.WithError(err).Info("could not find session")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		s.logger.WithField("user", usess).Info("logged in user")

		lat, err := strconv.ParseFloat(r.FormValue("lat"), 64)
		if err != nil {
			s.logger.WithError(err).Info("failed to convert lat from string to float")
		}
		lng, err := strconv.ParseFloat(r.FormValue("lng"), 64)
		if err != nil {
			s.logger.WithError(err).Info("failed to convert lat from string to float")
		}
		loc := session.Location{
			Lat: lat,
			Lng: lng,
		}

		usess.AddPhotoUpload(
			r.FormValue("url"),
			r.FormValue("datetime"),
			loc,
		)
		s.SessionStore.Create(usess)

		w.Header().Set("Location", "/composer")
		w.WriteHeader(http.StatusSeeOther)
		return
	}
}

func (s *server) HandleQueryPosts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// fetch cookie
		cookie, err := r.Cookie("sessionid")
		if err != nil {
			s.logger.WithError(err).Info("could not find sessionid cookie")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		// fetch session
		usess, err := s.SessionStore.FetchByID(cookie.Value)
		if err != nil {
			s.logger.WithError(err).Info("could not find session")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		s.logger.WithField("user", usess).Info("logged in user")

		// query post list
		afterKey := r.URL.Query().Get("after")
		postList, err := s.client.QueryPostList(usess.MicropubEndpoint, usess.AccessToken, afterKey)
		if err != nil {
			s.logger.WithError(err).Info("failed to query postlist")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// transform mf2 to jf2
		var postListView []mf2.MicroFormatView
		for _, postmf := range postList.Items {
			postListView = append(postListView, postmf.ToView())
		}
		if postList.Paging != nil {
			afterKey = postList.Paging.After
		}

		// query years list
		yearsList, err := s.client.QueryYearsList(usess.MicropubEndpoint, usess.AccessToken)
		if err != nil {
			s.logger.WithError(err).Info("failed to query year list")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s.logger.WithField("list", yearsList).Info("years list result")

		// render
		t, err := template.ParseFiles(
			"view/components.html",
			"view/layout.html",
			"view/postlist.html",
		)
		if err != nil {
			s.logger.WithError(err).Error("failed to parse templat files")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		outBuf := new(bytes.Buffer)
		v := struct {
			PageTitle string
			PostList  []mf2.MicroFormatView
			HasPaging bool
			AfterKey  string
			YearsList []mf2.ArchiveYear
		}{
			PageTitle: "LATEST POSTS",
			PostList:  postListView,
			HasPaging: postList.Paging != nil,
			AfterKey:  afterKey,
			YearsList: yearsList,
		}
		t.ExecuteTemplate(outBuf, "layout", v)

		w.Header().Set("Content-type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		w.Write(outBuf.Bytes())
	}
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

func (s *server) SubmitPost(
	sessionid,
	content,
	h string,
) HttpResponse {

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
	if usess.ComposerData.Location.HasLatLng() {
		formData.Add("location", usess.ComposerData.Location.ToGeoURL())
	}

	s.logger.WithFields(logrus.Fields{"request": formData}).Info("built micropub request")

	mpResponse, err := s.client.SendRequest(formData, usess.MicropubEndpoint, usess.AccessToken)
	if err != nil {
		s.logger.WithError(err).Error("failed to send MP request")
		return HttpResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}
	}

	// TODO redirect with message if not successful
	// TODO only clear session if successful
	usess.ClearComposerData()
	s.SessionStore.Create(usess)

	// redirect
	headers := map[string]string{
		"Location": mpResponse.Location,
	}
	return HttpResponse{
		StatusCode: mpResponse.StatusCode,
		Headers:    headers,
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

func (s *server) HandleAddLocationForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		cookie, err := r.Cookie("sessionid")
		if err != nil {
			s.logger.Infof("redirecting, could not find sessionid cookie")
			w.Header().Set("Location", "/login")
			w.WriteHeader(http.StatusSeeOther)
			return
		}

		s.logger.Infof("%s", r.RemoteAddr)

		response := HttpResponse{}

		switch r.Method {
		case "GET":
			response = s.ShowAddLocationForm(
				cookie.Value,
				r.URL.Query().Get("q"),
			)
		case "POST":
			response = s.AddLocation(
				cookie.Value,
				r.FormValue("locality"),
				r.FormValue("region"),
				r.FormValue("country"),
				r.FormValue("lat"),
				r.FormValue("lng"),
			)
		}

		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(response.StatusCode)
		w.Write([]byte(response.Body))
	}
}

func (s server) AddLocation(sessionid, locality, region, country, lat, lng string) HttpResponse {

	// checkSession
	usess, err := s.SessionStore.FetchByID(sessionid)
	if err != nil {
		return HttpResponse{
			StatusCode: http.StatusInternalServerError,
		}
	}
	s.logger.WithField("user", usess).
		Info("logged in user")

	location := session.Location{
		Locality: locality,
		Region:   region,
		Country:  country,
		Lat:      parseFloat(lat),
		Lng:      parseFloat(lng),
	}
	usess.AddLocation(location)

	err = s.SessionStore.Create(usess)
	if err != nil {
		s.logger.WithError(err).Error("failed to save session")
		return HttpResponse{StatusCode: http.StatusInternalServerError}
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

func parseFloat(f string) float64 {
	if s, err := strconv.ParseFloat(f, 64); err == nil {
		return s
	}
	return 0
}

type UploadedFile struct {
	Filename string
	File     io.Reader
}

type GeoURL string

func (url GeoURL) String() string {
	return string(url)
}

func (url GeoURL) Lat() float64 {
	if url.String() == "" {
		return 0
	}
	latlng := strings.Split(
		strings.TrimLeft(url.String(), "geo:"),
		",",
	)
	flt, err := strconv.ParseFloat(latlng[0], 64)
	if err != nil {
		return 0
	}
	return flt
}

func (url GeoURL) Lng() float64 {
	if url.String() == "" {
		return 0
	}
	latlng := strings.Split(
		strings.TrimLeft(url.String(), "geo:"),
		",",
	)
	flt, err := strconv.ParseFloat(latlng[1], 64)
	if err != nil {
		return 0
	}
	return flt
}

type MediaEndpointResponse struct {
	URL       string `json:"url"`
	Location  GeoURL `json:"location"`
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
		location := session.Location{
			Lat: res.Location.Lat(),
			Lng: res.Location.Lng(),
		}
		usess.AddPhotoUpload(res.URL, res.Published, location)
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

func (client Client) UploadToMediaServer(uploadedFile UploadedFile, usess session.UserSession) (MediaEndpointResponse, error) {
	// copy file to multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", uploadedFile.Filename)
	if err != nil {
		client.logger.WithError(err).Error("failed to create multipart")
		return MediaEndpointResponse{}, err
	}

	_, err = io.Copy(part, uploadedFile.File)
	if err != nil {
		client.logger.WithError(err).Error("failed to copy file into multipart")
		return MediaEndpointResponse{}, err
	}

	err = writer.Close()
	if err != nil {
		client.logger.WithError(err).Error("failed to close multipart writer")
		return MediaEndpointResponse{}, err
	}

	// create media-endpoint request
	req, err := http.NewRequest("POST", usess.MediaEndpoint, body)
	if err != nil {
		client.logger.WithError(err).Error("failed to create request")
		return MediaEndpointResponse{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+usess.AccessToken)

	// perform request
	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		client.logger.WithError(err).Error("failed to perform request")
		return MediaEndpointResponse{}, err
	}

	// read media-endpoint response
	respBody := &bytes.Buffer{}
	_, err = respBody.ReadFrom(resp.Body)
	if err != nil {
		client.logger.WithError(err).Error("failed to read response body")
		return MediaEndpointResponse{}, err
	}
	client.logger.
		WithField("response_body", respBody.String()).
		Info("media uploaded")

	mediaResponse := MediaEndpointResponse{}
	err = json.Unmarshal(respBody.Bytes(), &mediaResponse)
	if err != nil {
		client.logger.
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
		"view/media-thumbnail.html",
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
		Photos    []session.MediaUpload
		User      session.HCard
		Published string
		Location  string
	}{
		PageTitle: "Create Post",
		Photos:    usess.ComposerData.Photos,
		User:      usess.HCard,
		Published: usess.ComposerData.Published,
		Location:  usess.ComposerData.Location.ToHuman(),
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

func (client Client) QueryMediaList(
	mediaEndpoint,
	accessToken,
	afterKey,
	year,
	month string,
) (mpclient.MediaQueryListResponse, error) {
	var mediaResponse mpclient.MediaQueryListResponse

	mpURL := ""
	if afterKey == "" {
		mpURL = mediaEndpoint + "?q=source&limit=15&year=" + year + "&month=" + month
	} else {
		mpURL = mediaEndpoint + "?q=source&limit=15&after=" + afterKey
	}

	client.logger.
		WithField("media_endpoint", mpURL).
		Info("Querying media endpoint")

	req, err := http.NewRequest("GET", mpURL, nil)
	if err != nil {
		client.logger.WithError(err).Error("failed to create GET request")
		return mediaResponse, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		client.logger.WithError(err).Error("failed to perform GET request")
		return mediaResponse, err
	}
	respBody := &bytes.Buffer{}
	_, err = respBody.ReadFrom(resp.Body)
	if err != nil {
		client.logger.WithError(err).Error("failed to read GET response")
		return mediaResponse, err
	}

	decoder := json.NewDecoder(respBody)
	err = decoder.Decode(&mediaResponse)
	if err != nil {
		client.logger.WithError(err).Error("failed to decode json body")
		return mediaResponse, err
	}

	return mediaResponse, nil
}

func (client Client) QueryMediaURL(
	URL,
	mediaEndpoint,
	accessToken string,
) (mpclient.MediaQueryListResponseItem, error) {
	var mediaResponse mpclient.MediaQueryListResponseItem

	client.logger.
		WithField("media_endpoint", mediaEndpoint).
		Info("Querying media endpoint")

	req, err := http.NewRequest("GET", mediaEndpoint+"?q=source&url="+URL, nil)
	if err != nil {
		client.logger.WithError(err).Error("failed to create GET request")
		return mediaResponse, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		client.logger.WithError(err).Error("failed to perform GET request")
		return mediaResponse, err
	}
	respBody := &bytes.Buffer{}
	_, err = respBody.ReadFrom(resp.Body)
	if err != nil {
		client.logger.WithError(err).Error("failed to read GET response")
		return mediaResponse, err
	}

	decoder := json.NewDecoder(respBody)
	err = decoder.Decode(&mediaResponse)
	if err != nil {
		client.logger.WithError(err).Error("failed to decode json body")
		return mediaResponse, err
	}
	client.logger.Infof("%+v", mediaResponse)

	return mediaResponse, nil
}

func (client Client) QueryPostList(micropubEndpoint, accessToken, afterKey string) (mf2.PostList, error) {
	var postList mf2.PostList
	var mpURL string

	if afterKey == "" {
		mpURL = micropubEndpoint + "?q=source"
	} else {
		mpURL = micropubEndpoint + "?q=source&after=" + afterKey
	}

	client.logger.WithField("endpoint", mpURL).Info("Querying endpoint")
	req, err := http.NewRequest("GET", mpURL, nil)

	if err != nil {
		return postList, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		client.logger.WithError(err).Error("failed to perform GET request")
		return postList, err
	}
	respBody := &bytes.Buffer{}
	_, err = respBody.ReadFrom(resp.Body)
	if err != nil {
		client.logger.WithError(err).Error("failed to read GET request body")
		return postList, err
	}
	// parse response
	decoder := json.NewDecoder(respBody)
	err = decoder.Decode(&postList)
	if err != nil {
		client.logger.WithError(err).Error("failed to decode json")
		return postList, err
	}

	return postList, nil
}

func (client Client) QueryYearsList(micropubEndpoint, accessToken string) ([]mf2.ArchiveYear, error) {
	var postList []mf2.ArchiveYear

	mpURL := micropubEndpoint + "?q=years"

	client.logger.WithField("endpoint", mpURL).Info("Querying endpoint")
	req, err := http.NewRequest("GET", mpURL, nil)

	if err != nil {
		return postList, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		client.logger.WithError(err).Error("failed to perform GET request")
		return postList, err
	}
	respBody := &bytes.Buffer{}
	_, err = respBody.ReadFrom(resp.Body)
	if err != nil {
		client.logger.WithError(err).Error("failed to read GET request body")
		return postList, err
	}
	// parse response
	decoder := json.NewDecoder(respBody)
	err = decoder.Decode(&postList)
	if err != nil {
		client.logger.WithError(err).Error("failed to decode json")
		return postList, err
	}

	return postList, nil
}

func (client Client) QueryMonthsList(micropubEndpoint, accessToken, currentYear string) ([]mf2.ArchiveMonth, error) {
	var postList []mf2.ArchiveMonth

	mpURL := fmt.Sprintf("%s?q=months&year=%s", micropubEndpoint, currentYear)
	client.logger.WithField("endpoint", mpURL).Info("Querying endpoint")
	req, err := http.NewRequest("GET", mpURL, nil)

	if err != nil {
		return postList, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		client.logger.WithError(err).Error("failed to perform GET request")
		return postList, err
	}
	respBody := &bytes.Buffer{}
	_, err = respBody.ReadFrom(resp.Body)
	if err != nil {
		client.logger.WithError(err).Error("failed to read GET request body")
		return postList, err
	}
	// parse response
	decoder := json.NewDecoder(respBody)
	err = decoder.Decode(&postList)
	if err != nil {
		client.logger.WithError(err).Error("failed to decode json")
		return postList, err
	}

	return postList, nil
}

func (client Client) SendRequest(body url.Values, mpEndpoint, bearerToken string) (MicropubEndpointResponse, error) {

	req, err := http.NewRequest("POST", mpEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		client.logger.WithError(err).Error("failed to create request")
		return MicropubEndpointResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// perform request
	client.logger.WithField("micropub_endpoint", mpEndpoint).Info("sending micropub request")
	httpclient := &http.Client{}
	resp, err := httpclient.Do(req)
	if err != nil {
		client.logger.WithError(err).Error("failed to perform request")
		return MicropubEndpointResponse{}, err
	}
	client.logger.WithField("micropub_response", resp.StatusCode).Info("micropub response")

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

func (s *server) ShowAddLocationForm(sessionid, locationQuery string) HttpResponse {

	// checkSession
	usess, err := s.SessionStore.FetchByID(sessionid)
	if err != nil {
		return HttpResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}
	}
	s.logger.WithFields(logrus.Fields{"user": usess}).Info("logged in user")

	locations := s.geocoder.Lookup(locationQuery)

	// render
	t, err := template.ParseFiles(
		"view/components.html",
		"view/layout.html",
		"view/addlocation.html",
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
		Locations []session.Location
	}{
		PageTitle: "Add Location",
		Locations: locations,
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
