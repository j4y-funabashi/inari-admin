package indieauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/j4y_funabashi/inari-admin/indieauth"
	"github.com/j4y_funabashi/inari-admin/storage"
	"github.com/sirupsen/logrus"
)

type mockSessionStore struct {
	tokenEndpoint string
	userSession   storage.UserSession
}

func (s mockSessionStore) Create(usess storage.UserSession) error {
	return nil
}

func (s mockSessionStore) FetchByID(postID string) (storage.UserSession, error) {
	sess := s.userSession
	sess.TokenEndpoint = s.tokenEndpoint
	return sess, nil
}

func TestCallback(t *testing.T) {
	tt := []struct {
		name           string
		state          string
		code           string
		clientID       string
		redirectURL    string
		userSession    storage.UserSession
		tokenServerRes func(w http.ResponseWriter, r *http.Request)
	}{
		{
			name:        "hello",
			state:       "",
			code:        "",
			clientID:    "",
			redirectURL: "",
			userSession: storage.UserSession{Me: "http://example.com/jay"},
			tokenServerRes: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"me": "https://jay.example.com", "client_id": "test", "scope": "create"}`))
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	logger := logrus.New()

	for _, tc := range tt {

		server := httptest.NewServer(http.HandlerFunc(tc.tokenServerRes))
		sessionStore := mockSessionStore{
			tokenEndpoint: server.URL,
			userSession:   tc.userSession,
		}
		client := indieauth.NewClient(
			"",
			sessionStore,
			logger,
		)

		response := client.Callback(
			tc.state,
			tc.code,
			tc.clientID,
			tc.redirectURL,
		)

		t.Errorf("%+v", response)
	}
}
