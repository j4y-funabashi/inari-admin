package login

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/j4y_funabashi/inari-admin/pkg/indieauth"
	"github.com/sirupsen/logrus"
)

func NewServer(logger *logrus.Logger, authClient indieauth.Client, clientID, redirectURL string) server {
	s := server{
		logger:      logger,
		authClient:  authClient,
		clientID:    clientID,
		redirectURL: redirectURL,
	}
	return s
}

type server struct {
	logger      *logrus.Logger
	authClient  indieauth.Client
	redirectURL string
	clientID    string
}

type HttpResponse struct {
	Headers    map[string]string
	Body       string
	StatusCode int
}

func (s *server) Routes(router *mux.Router) {
	router.HandleFunc("/login", s.HandleLogin())
	router.HandleFunc("/login-init", s.HandleLoginInit())
	router.HandleFunc("/login-callback", s.HandleLoginCallback())
}

func (s *server) HandleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := s.ShowLoginForm()
		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(response.StatusCode)
		w.Write([]byte(response.Body))
	}
}

func (s *server) HandleLoginCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		err := r.ParseForm()
		if err != nil {
			s.logger.WithError(err).Error("failed to parse form")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s.logger.WithFields(logrus.Fields{
			"request": r.Form,
		}).Info("callback received")

		state := r.Form.Get("state")
		code := r.Form.Get("code")

		response := s.LoginCallback(state, code)

		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(response.StatusCode)
		w.Write([]byte(response.Body))
	}
}

func (s *server) HandleLoginInit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		me := r.FormValue("me")

		response := s.InitLogin(me)
		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(response.StatusCode)
		w.Write([]byte(response.Body))
	}
}

func (s *server) ShowLoginForm() HttpResponse {
	t, err := template.ParseFiles(
		"view/components.html",
		"view/layout.html",
		"view/login.html",
	)
	if err != nil {
		return HttpResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       err.Error(),
		}
	}

	w := new(bytes.Buffer)
	v := struct{ PageTitle string }{PageTitle: "Login"}
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

func (s *server) InitLogin(me string) HttpResponse {

	s.logger.WithFields(logrus.Fields{
		"me": me,
	}).Info("initializing login")

	response := s.authClient.Init(
		me,
		s.clientID,
		s.redirectURL,
	)
	s.logger.Infof("indieauth response %v", response)

	return HttpResponse{
		StatusCode: response.StatusCode,
		Headers:    response.Headers,
		Body:       response.Body,
	}
}

func (s *server) LoginCallback(state, code string) HttpResponse {
	response := s.authClient.Callback(
		state,
		code,
		s.clientID,
		s.redirectURL,
	)
	return HttpResponse{
		StatusCode: response.StatusCode,
		Headers:    response.Headers,
		Body:       response.Body,
	}
}
