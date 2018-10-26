package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/tomnomnom/linkheader"
	"golang.org/x/net/html"
	"willnorris.com/go/microformats"

	uuid "github.com/satori/go.uuid"
)

type SessionStore interface {
	Create(usess UserSession) error
	FetchByID(postID string) (UserSession, error)
}

type UserSession struct {
	Uid                   string       `json:"uid"`
	Me                    string       `json:"me"`
	ClientId              string       `json:"client_id"`
	RedirectUri           string       `json:"redirect_uri"`
	Scope                 string       `json:"scope"`
	State                 string       `json:"state"`
	AuthorizationEndpoint string       `json:"authorization_endpoint"`
	TokenEndpoint         string       `json:"token_endpoint"`
	MicropubEndpoint      string       `json:"micropub_endpoint"`
	MediaEndpoint         string       `json:"media_endpoint"`
	AccessToken           string       `json:"access_token"`
	TokenType             string       `json:"token_type"`
	ComposerData          ComposerData `json:"composer_data"`
	HCard                 HCard        `json:"h_card"`
}

type MediaUpload struct {
	URL       string `json:"url"`
	Published string `json:"published"`
	Location  string `json:"location"`
}

type ComposerData struct {
	Photos    []MediaUpload `json:"photos"`
	Published string
}

func (usess *UserSession) AddPhotoUpload(url, pub, loc string) {
	usess.ComposerData.Photos = append(
		usess.ComposerData.Photos,
		MediaUpload{
			URL:       url,
			Published: pub,
			Location:  loc,
		},
	)
	if pub != "" {
		usess.ComposerData.Published = pub
	}
}

func (usess *UserSession) ClearComposerData() {
	usess.ComposerData = ComposerData{}
}

func NewUserSession(me, clientId, redirectUri string) (UserSession, error) {
	p := UserSession{}
	uid, err := uuid.NewV4()
	if err != nil {
		return p, err
	}
	p.Uid = uid.String()
	p.Me = me
	p.ClientId = clientId
	p.RedirectUri = redirectUri
	p.Scope = "create"
	p.State = uid.String()
	return p, nil
}

func (params *UserSession) BuildAuthRedirectUrl() (string, error) {
	authUrl, err := url.Parse(params.AuthorizationEndpoint)
	if err != nil {
		return "", err
	}
	q := authUrl.Query()
	q.Set("me", params.Me)
	q.Set("client_id", params.ClientId)
	q.Set("redirect_uri", params.RedirectUri)
	q.Set("state", params.State)
	q.Set("scope", params.Scope)
	q.Set("response_type", "code")
	authUrl.RawQuery = q.Encode()
	return authUrl.String(), nil
}

func (usess *UserSession) DiscoverEndpoints() error {
	resp, err := http.Get(usess.Me)
	if err != nil {
		log.Printf("failed to GET [%s][%s]", usess.Me, err.Error())
		return err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("URL returned a non-200: [%s][%d]", usess.Me, resp.StatusCode)
		return fmt.Errorf("URL returned a non-200")
	}
	doc, err := html.Parse(resp.Body)
	if err != nil {
		log.Printf("failed to parse HTML [%s][%s]", usess.Me, err.Error())
		return fmt.Errorf("failed to parse HTML [%s][%s]", usess.Me, err.Error())
	}

	usess.AuthorizationEndpoint = findEndpoint(doc, "authorization_endpoint", resp.Header)
	if usess.AuthorizationEndpoint == "" {
		return fmt.Errorf("failed to find authorization_endpoint")
	}
	usess.TokenEndpoint = findEndpoint(doc, "token_endpoint", resp.Header)
	usess.MicropubEndpoint = findEndpoint(doc, "micropub", resp.Header)
	usess.discoverMediaEndpoint()

	// try to find h-card
	usess.HCard = discoverHcard(usess.Me)

	return nil
}

type HCard struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Photo string `json:"photo"`
}

func discoverHcard(meURL string) HCard {
	resp, err := http.Get(meURL)
	if err != nil {
		log.Printf("failed to GET [%s][%s]", meURL, err.Error())
		return HCard{}
	}
	pURL, err := url.Parse(meURL)
	if err != nil {
		log.Printf("failed to parse URL [%s]", err.Error())
		return HCard{}
	}
	mf := microformats.Parse(resp.Body, pURL)
	log.Printf("mf2: %+v", mf)
	for _, item := range mf.Items {
		if sliceContains(item.Type, "h-card") {
			if isRepresentativeHcard(item, meURL) {
				return HCard{
					Name:  mfGetFirstString(item.Properties["name"]),
					URL:   mfGetFirstString(item.Properties["url"]),
					Photo: mfGetFirstString(item.Properties["photo"]),
				}
			}
		}
	}
	return HCard{}
}

func isRepresentativeHcard(mf *microformats.Microformat, meURL string) bool {
	if mfSliceContains(mf.Properties["url"], meURL) == false {
		return false
	}
	if mfSliceContains(mf.Properties["uid"], meURL) == false {
		return false
	}
	return true
}

func mfGetFirstString(property []interface{}) string {
	for _, val := range property {
		if v, ok := val.(string); ok == true {
			return v
		}
	}
	return ""
}

func mfSliceContains(property []interface{}, value string) bool {
	for _, val := range property {
		if v, ok := val.(string); ok == true {
			if v == value {
				return true
			}
		}
	}
	return false
}

func sliceContains(slice []string, value string) bool {
	for _, v := range slice {
		if strings.ToLower(v) == strings.ToLower(value) {
			return true
		}
	}
	return false
}

func (usess *UserSession) discoverMediaEndpoint() {
	usess.MediaEndpoint = ""
	if usess.MicropubEndpoint == "" {
		return
	}
	configUrl, err := buildConfigUrl(usess.MicropubEndpoint)
	if err != nil {
		return
	}
	config, err := fetchMicropubConfig(configUrl)
	if err != nil {
		log.Printf("failed to fetch micropub config: %v", err)
		return
	}
	usess.MediaEndpoint = config.MediaEndpoint
}

func buildConfigUrl(micropubEndpoint string) (string, error) {
	u, err := url.Parse(micropubEndpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("q", "config")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

type MicropubConfig struct {
	MediaEndpoint string `json:"media-endpoint"`
}

func fetchMicropubConfig(configUrl string) (MicropubConfig, error) {
	config := MicropubConfig{}
	resp, err := http.Get(configUrl)
	if err != nil {
		return config, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(body, &config); err != nil {
		return config, err
	}
	return config, nil
}

func findEndpoint(n *html.Node, endpoint string, head http.Header) (out string) {
	if head.Get("Link") != "" {
		for k, v := range head {
			if strings.TrimSpace(strings.ToLower(k)) == "link" {
				for _, v2 := range v {
					for _, link := range linkheader.Parse(v2) {
						if link.Rel == endpoint {
							return link.URL
						}
					}
				}
			}
		}
		return ""
	}
	if n.Type == html.ElementNode && n.Data == "link" {
		for _, a := range n.Attr {
			if a.Key == "rel" && a.Val == endpoint {
				for _, b := range n.Attr {
					if b.Key == "href" {
						return b.Val
					}
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = findEndpoint(c, endpoint, head)
		if out != "" {
			return out
		}
	}
	return out
}

type s3SessionStore struct {
	downloader *s3manager.Downloader
	uploader   *s3manager.Uploader
	bucket     string
}

func (s s3SessionStore) Create(usess UserSession) error {
	key := "sessions/" + usess.Uid + ".json"

	data := new(bytes.Buffer)
	err := json.NewEncoder(data).Encode(usess)
	if err != nil {
		return fmt.Errorf("failed to encode json %v", err)
	}

	_, err = s.uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   data,
		ACL:    aws.String("private"),
	})
	return err
}

func (s s3SessionStore) FetchByID(sessionID string) (UserSession, error) {

	key := "sessions/" + sessionID + ".json"
	var sess UserSession

	in := s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	buf := aws.NewWriteAtBuffer([]byte{})
	_, err := s.downloader.Download(buf, &in)
	if err != nil {
		return sess, err
	}

	err = json.Unmarshal(buf.Bytes(), &sess)

	return sess, nil
}

func NewS3SessionStore(region, bucket string) (SessionStore, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		return s3SessionStore{}, err
	}
	downloader := s3manager.NewDownloader(sess)
	uploader := s3manager.NewUploader(sess)
	return s3SessionStore{downloader: downloader, uploader: uploader, bucket: bucket}, nil
}
