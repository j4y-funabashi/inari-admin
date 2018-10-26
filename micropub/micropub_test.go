package micropub_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/j4y_funabashi/inari-admin/micropub"
	"github.com/j4y_funabashi/inari-admin/pkg/session"
	"github.com/matryer/is"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

type mockSessionStore struct {
	createResponse    func(usess session.UserSession) error
	fetchByIDResponse func(postID string) (session.UserSession, error)
}

type mockClient struct {
	uploadResponse func(uploadedFile micropub.UploadedFile, usess session.UserSession) (micropub.MediaEndpointResponse, error)
	sendRequest    func(body url.Values, endpoint, bearerToken string) (micropub.MicropubEndpointResponse, error)
}

func (mc mockClient) UploadToMediaServer(uploadedFile micropub.UploadedFile, usess session.UserSession) (micropub.MediaEndpointResponse, error) {
	return mc.uploadResponse(uploadedFile, usess)
}

func (mc mockClient) SendRequest(body url.Values, endpoint, bearerToken string) (micropub.MicropubEndpointResponse, error) {
	return mc.sendRequest(body, endpoint, bearerToken)
}

func (sstore mockSessionStore) Create(usess session.UserSession) error {
	return sstore.createResponse(usess)
}

func (sstore mockSessionStore) FetchByID(postID string) (session.UserSession, error) {
	return sstore.fetchByIDResponse(postID)
}

func TestAddPhotos(t *testing.T) {

	is := is.NewRelaxed(t)
	goodUploadedFile := micropub.UploadedFile{Filename: "", File: strings.NewReader("")}
	expectedPhotos := []session.MediaUpload{
		{
			URL:       "https://example.com/1.jpg",
			Published: "2010-01-28",
			Location:  "leeds",
		},
	}
	mediaResponse := micropub.MediaEndpointResponse{
		URL:       "https://example.com/1.jpg",
		Published: "2010-01-28",
		Location:  "leeds",
	}

	var tests = []struct {
		name              string
		createResponse    func(usess session.UserSession) error
		fetchByIDResponse func(postID string) (session.UserSession, error)
		uploadResponse    func(uploadedFile micropub.UploadedFile, usess session.UserSession) (micropub.MediaEndpointResponse, error)
		expected          micropub.HttpResponse
		fileList          []micropub.UploadedFile
	}{
		{
			name: "happy paths",
			fetchByIDResponse: func(postID string) (session.UserSession, error) {
				return session.UserSession{}, nil
			},
			createResponse: func(usess session.UserSession) error {

				if len(usess.ComposerData.Photos) != len(expectedPhotos) {
					t.Errorf("session should contain %d photo, found %d", len(expectedPhotos), len(usess.ComposerData.Photos))
				}

				for key, photo := range usess.ComposerData.Photos {
					if expectedPhotos[key] != photo {
						t.Errorf("expected photo %d to be %+v got %+v", key, expectedPhotos[key], photo)
					}
				}
				is.Equal(usess.ComposerData.Published, "2010-01-28")
				is.Equal(usess.ComposerData.Location, "leeds")

				return nil
			},
			uploadResponse: func(uploadedFile micropub.UploadedFile, usess session.UserSession) (micropub.MediaEndpointResponse, error) {
				return mediaResponse, nil
			},
			expected: micropub.HttpResponse{StatusCode: 303, Headers: map[string]string{"Location": "/composer"}},
			fileList: []micropub.UploadedFile{goodUploadedFile},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			logger, hook := test.NewNullLogger()
			sstore := mockSessionStore{
				createResponse:    tt.createResponse,
				fetchByIDResponse: tt.fetchByIDResponse,
			}
			mpClient := mockClient{
				uploadResponse: tt.uploadResponse,
			}
			sessionID := "1234"

			mpServer := micropub.NewServer(logger, sstore, mpClient)
			res := mpServer.AddPhotos(sessionID, tt.fileList)

			is.Equal(res, tt.expected)
			hook.Reset()
		})
	}
}

func TestSendRequest(t *testing.T) {

	is := is.NewRelaxed(t)
	var tests = []struct {
		name        string
		expectedErr error
		expectedRes micropub.MicropubEndpointResponse
	}{
		{
			name:        "happy path",
			expectedErr: nil,
			expectedRes: micropub.MicropubEndpointResponse{
				StatusCode: 201},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			// arrange
			logger := logrus.New()
			mpClient := micropub.NewClient(logger)
			mpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				err := r.ParseForm()
				if err != nil {
					t.Errorf("failed to parse form: %s", err.Error())
				}
				is.Equal(r.Header.Get("Authorization"), "Bearer chickentoken")
				is.Equal(r.Header.Get("Content-type"), "application/x-www-form-urlencoded")
				is.Equal(r.FormValue("content"), "hellchickencontent")
				for key, photourl := range r.Form["photo"] {
					expectedURL := fmt.Sprintf("http://example.com/%d.jpg", key+1)
					is.Equal(expectedURL, photourl)
				}

				w.WriteHeader(201)
			}))

			accessToken := "chickentoken"
			formData := url.Values{}
			formData.Add("content", "hellchickencontent")
			formData.Add("photo", "http://example.com/1.jpg")
			formData.Add("photo", "http://example.com/2.jpg")

			// act
			res, err := mpClient.SendRequest(formData, mpServer.URL, accessToken)

			// assert
			is.Equal(tt.expectedRes, res)
			is.Equal(tt.expectedErr, err)
		})
	}
}

func TestUploadToMediaServer(t *testing.T) {
	var tests = []struct {
		name             string
		uploadedFile     micropub.UploadedFile
		expectedResponse micropub.MediaEndpointResponse
		expectedErr      error
	}{
		{
			name: "happy path",
			uploadedFile: micropub.UploadedFile{
				Filename: "test.jpg",
				File:     strings.NewReader("testy test"),
			},
			expectedResponse: micropub.MediaEndpointResponse{
				URL:       "http://example.com/test.jpg",
				Location:  "leeds",
				Published: "2010-01-28 10:00:00",
			},
			expectedErr: nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			is := is.New(t)

			logger := logrus.New()
			mpClient := micropub.NewClient(logger)
			mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "http://example.com/test.jpg")
				jsonres := struct {
					Published string `json:"published"`
					Location  string `json:"location"`
				}{
					Published: "2010-01-28 10:00:00",
					Location:  "leeds",
				}
				json.NewEncoder(w).Encode(jsonres)
			}))
			usess := session.UserSession{
				MediaEndpoint: mediaServer.URL,
				AccessToken:   "123testtoken",
			}

			// act
			res, err := mpClient.UploadToMediaServer(tt.uploadedFile, usess)

			// assert
			is.Equal(tt.expectedResponse, res)
			is.Equal(tt.expectedErr, err)
		})
	}
}

func TestSubmitPost(t *testing.T) {

	is := is.NewRelaxed(t)
	var tests = []struct {
		name              string
		content           string
		h                 string
		createResponse    func(usess session.UserSession) error
		fetchByIDResponse func(postID string) (session.UserSession, error)
		sendRequest       func(body url.Values, endpoint, bearerToken string) (micropub.MicropubEndpointResponse, error)
		expected          micropub.HttpResponse
		fileList          []micropub.UploadedFile
	}{
		{
			name:           "happy paths",
			content:        "hellchicken content",
			h:              "entry",
			createResponse: func(usess session.UserSession) error { return nil },
			fetchByIDResponse: func(postID string) (session.UserSession, error) {
				return session.UserSession{
					MicropubEndpoint: "http://micropub.example.com",
					AccessToken:      "hellchickentoken",
					ComposerData: session.ComposerData{
						Photos: []session.MediaUpload{
							{URL: "http://example.com/1.jpg"},
							{URL: "http://example.com/2.jpg"},
							{URL: "http://example.com/3.jpg"},
						},
						Published: "2018-01-28T10:47:54+01:00",
						Location:  "leedz",
					},
				}, nil
			},
			expected: micropub.HttpResponse{},
			sendRequest: func(body url.Values, endpoint, bearerToken string) (micropub.MicropubEndpointResponse, error) {
				is.Equal("hellchicken content", body.Get("content"))
				is.Equal("entry", body.Get("h"))
				is.Equal("http://micropub.example.com", endpoint)
				is.Equal("hellchickentoken", bearerToken)

				is.Equal(body["photo"][0], "http://example.com/1.jpg")
				is.Equal(body["photo"][1], "http://example.com/2.jpg")
				is.Equal(body["photo"][2], "http://example.com/3.jpg")
				is.Equal(body.Get("published"), "2018-01-28T10:47:54+01:00")
				is.Equal(body.Get("location"), "leedz")

				return micropub.MicropubEndpointResponse{}, nil
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			// arrange
			logger, hook := test.NewNullLogger()
			sstore := mockSessionStore{
				createResponse:    tt.createResponse,
				fetchByIDResponse: tt.fetchByIDResponse,
			}
			mpClient := mockClient{sendRequest: tt.sendRequest}
			sessionID := "1234"
			mpServer := micropub.NewServer(logger, sstore, mpClient)

			// act
			res := mpServer.SubmitPost(sessionID, tt.content, tt.h)
			t.Errorf("%+v", res)

			// assert
			hook.Reset()
		})
	}
}
