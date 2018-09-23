package micropub

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/j4y_funabashi/inari/storage"
)

func NewServer(logger *logrus.Logger) server {
	s := server{
		logger: logger,
	}
	return s
}

type server struct {
	logger       *logrus.Logger
	SessionStore storage.SessionStore
}

type HttpResponse struct {
	Headers    map[string]string
	Body       string
	StatusCode int
}

func (s *server) Routes(router *mux.Router) {
	router.HandleFunc("/new", s.HandleNewPostForm())
}

func (s *server) HandleNewPostForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		cookie, err := r.Cookie("sessionid")
		if err != nil {
			w.Header().Set("Location", "/login")
			w.WriteHeader(http.StatusSeeOther)
			return
		}

		response := s.ShowNewPostForm(cookie.Value)
		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(response.StatusCode)
		w.Write([]byte(response.Body))
	}
}

func (s *server) ShowNewPostForm(sessionid string) HttpResponse {

	// fetch session

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
	v := struct{ PageTitle string }{PageTitle: "Create Post"}
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
