package main

import (
	"html/template"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func main() {

	// config
	port := "80"

	// deps
	logger := logrus.New()
	logger.Formatter = &logrus.JSONFormatter{}
	router := mux.NewRouter()

	router.HandleFunc("/", handleHomepage(logger))
	router.HandleFunc("/auth", handleAuth(logger))
	router.HandleFunc("/token", handleToken(logger))
	router.HandleFunc("/micropub", handleMicropub(logger))

	logger.Info("mp mock server running on port " + port)
	logger.Fatal(http.ListenAndServe(":"+port, router))
}

func handleHomepage(logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.
			Info("homepage recieved request")

		// render
		t, err := template.ParseFiles("home.html")
		if err != nil {
			logger.WithError(err).Error("failed to parse template file")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		v := struct {
			BaseURL string
		}{
			BaseURL: "http://mpserver",
		}
		t.ExecuteTemplate(w, "layout", v)

		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(200)
		return
	}
}

func handleAuth(logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.
			Info("auth recieved request")

		redirectURL, err := url.Parse(r.URL.Query().Get("redirect_uri"))
		if err != nil {
			logger.WithError(err).Error("failed to parse redirect url")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		q := redirectURL.Query()
		q.Add("state", r.URL.Query().Get("state"))
		q.Add("code", "666")
		redirectURL.RawQuery = q.Encode()

		w.Header().Set("Location", redirectURL.String())
		w.WriteHeader(http.StatusSeeOther)
		return
	}
}

func handleToken(logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.
			Info("token recieved request")
	}
}

func handleMicropub(logger *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.
			Info("micropub recieved request")
	}
}
