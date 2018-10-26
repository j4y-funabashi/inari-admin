package indieauth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/j4y_funabashi/inari-admin/pkg/session"
	"github.com/j4y_funabashi/inari-admin/responder"
	"github.com/sirupsen/logrus"
)

type TokenEndpoint interface {
	VerifyAccessToken(bearerToken string) TokenResponse
}

type tokenEndpoint struct {
	URL string
}

type TokenResponse struct {
	Me               string `json:"me"`
	ClientId         string `json:"client_id"`
	Scope            string `json:"scope"`
	IssuedBy         string `json:"issued_by"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	StatusCode       int
}

func (tr TokenResponse) IsValid() bool {
	if tr.StatusCode != 200 {
		return false
	}
	if strings.TrimSpace(tr.Me) == "" {
		return false
	}
	if strings.TrimSpace(tr.Scope) == "" {
		return false
	}
	return true
}

type Client interface {
	VerifyAccessToken(bearerToken string) (TokenResponse, error)
	Init(me, clientId, redirectUri string) responder.Response
	Callback(state, code, clientId, redirectUri string) responder.Response
}

func NewClient(tokenEndpoint string, sessionStore session.SessionStore, logger *logrus.Logger) Client {
	return client{
		TokenEndpoint: tokenEndpoint,
		SessionStore:  sessionStore,
		logger:        logger,
	}
}

type client struct {
	TokenEndpoint string
	SessionStore  session.SessionStore
	logger        *logrus.Logger
}

func (client client) VerifyAccessToken(bearerToken string) (TokenResponse, error) {
	req, err := http.NewRequest("GET", client.TokenEndpoint, nil)
	if err != nil {
		log.Printf("failed to create GET request: %s", err.Error())
		return TokenResponse{}, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", bearerToken)
	req.Header.Add("Accept", "application/json")

	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		log.Printf("failed to GET token endpoint: %s", err.Error())
		return TokenResponse{}, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read response body: %s", err.Error())
		return TokenResponse{}, err
	}

	tokenRes := TokenResponse{StatusCode: resp.StatusCode}
	err = json.Unmarshal(body, &tokenRes)
	if err != nil {
		log.Printf("failed to unmarshal response body: %s", err.Error())
		return TokenResponse{}, err
	}
	return tokenRes, nil
}

func (client client) Init(me, clientID, redirectURI string) responder.Response {
	var res responder.Response
	usess, err := session.NewUserSession(
		me,
		clientID,
		redirectURI,
	)
	if err != nil {
		client.logger.WithError(err).Error("failed to create user session")
		res.StatusCode = http.StatusBadRequest
		res.Body = err.Error()
		return res
	}
	err = usess.DiscoverEndpoints()
	if err != nil {
		client.logger.WithError(err).Error("failed to discover endpoints")
		res.StatusCode = http.StatusBadRequest
		res.Body = err.Error()
		return res
	}
	authUrl, err := usess.BuildAuthRedirectUrl()
	if err != nil {
		client.logger.WithError(err).Error("failed to build auth url")
		return res
	}

	err = client.SessionStore.Create(usess)
	if err != nil {
		client.logger.WithError(err).Error("failed to save session")
		res.StatusCode = http.StatusInternalServerError
		return res
	}

	headers := map[string]string{
		"Location": authUrl,
	}
	res.Headers = headers
	res.StatusCode = http.StatusSeeOther

	return res
}

type VerifyCodeResponse struct {
	Me          string `json:"me"`
	Scope       string `json:"scope"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func (vr VerifyCodeResponse) CompareDomains(other string) error {
	d1 := vr.parseDomain(vr.Me)
	d2 := vr.parseDomain(other)
	log.Printf("d1:: %s -> %s", vr.Me, d1)
	log.Printf("d2:: %s -> %s", other, d2)
	if strings.ToLower(d1) != strings.ToLower(d2) {
		return fmt.Errorf("domains do not match %s != %s", d1, d2)
	}
	return nil
}
func (vr VerifyCodeResponse) parseDomain(dom string) string {
	me, err := url.Parse(dom)
	if err != nil {
		return ""
	}
	parts := strings.Split(me.Hostname(), ".")
	domain := parts[len(parts)-2] + "." + parts[len(parts)-1]
	return domain
}

func (client client) Callback(state, code, clientId, redirectUri string) responder.Response {
	var res responder.Response

	// FETCH USER SESSION
	s, err := client.SessionStore.FetchByID(state)
	if err != nil {
		log.Printf("failed to fetch session: %+v", err)
		res.StatusCode = http.StatusInternalServerError
		return res
	}
	// VERIFY USER SESSION
	if s.State != state {
		log.Printf("state values did not match: [%+v] !== [%+v]", s.State, state)
		res.StatusCode = http.StatusForbidden
		return res
	}
	log.Printf("user session: %+v", s)

	// AUTHORIZATION CODE VERIFICATION
	// BUILD POST REQ
	httpclient := &http.Client{}
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", clientId)
	data.Set("redirect_uri", redirectUri)
	data.Set("me", s.Me)
	req, err := http.NewRequest("POST", s.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		log.Printf("failed to build verify code request: %+v", err)
		res.StatusCode = http.StatusInternalServerError
		return res
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpclient.Do(req)
	if err != nil {
		log.Printf("failed to POST to TokenEndpoint: [%s][%+v]", s.TokenEndpoint, err)
		res.StatusCode = http.StatusInternalServerError
		return res
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("TokenEndpoint returned a non-200: [%s][%d]", s.TokenEndpoint, resp.StatusCode)
		res.StatusCode = http.StatusForbidden
		return res
	}
	log.Printf("verify code response: %+v", resp)
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		log.Printf("failed to read verify code response: %+v", err)
		res.StatusCode = http.StatusInternalServerError
		return res
	}
	var verifyRes VerifyCodeResponse
	json.Unmarshal(body, &verifyRes)
	log.Printf("verify code response: %+v", verifyRes)

	err = verifyRes.CompareDomains(s.Me)
	if err != nil {
		log.Printf("%s", err)
		res.StatusCode = http.StatusForbidden
		return res
	}

	s.AccessToken = verifyRes.AccessToken
	s.TokenType = verifyRes.TokenType
	err = client.SessionStore.Create(s)
	if err != nil {
		log.Printf("failed to save session: %+v", err)
		res.StatusCode = http.StatusInternalServerError
		return res
	}

	cookie := fmt.Sprintf("sessionid=%s; Path=/", s.Uid)
	headers := map[string]string{
		"Location":   "/composer",
		"Set-Cookie": cookie,
	}
	res.StatusCode = http.StatusSeeOther
	res.Headers = headers

	return res
}
