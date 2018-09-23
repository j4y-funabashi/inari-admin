package storage

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/j4y_funabashi/inari/mf2"
	"github.com/tomnomnom/linkheader"
	"golang.org/x/net/html"

	_ "github.com/mattn/go-sqlite3"
	uuid "github.com/satori/go.uuid"
)

type PostStore interface {
	CreatePost(mf mf2.MicroFormat) error
	FetchByID(postID string) (mf2.MicroFormatView, error)
	FetchFeed(feedID string, limit int) (mf2.MicroFormatView, error)
}

type dynamoPostStore struct {
	TableName     string
	FeedTableName string
	Db            *dynamodb.DynamoDB
}

type sqliteStore struct {
	Db *sql.DB
}

type SessionStore interface {
	Create(usess UserSession) error
	FetchByID(postID string) (UserSession, error)
}
type dynamoSessionStore struct {
	TableName string
	Db        *dynamodb.DynamoDB
}
type UserSession struct {
	Uid                   string `json:"uid"`
	Me                    string `json:"me"`
	ClientId              string `json:"client_id"`
	RedirectUri           string `json:"redirect_uri"`
	Scope                 string `json:"scope"`
	State                 string `json:"state"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	MicropubEndpoint      string `json:"micropub_endpoint"`
	MediaEndpoint         string `json:"media_endpoint"`
	AccessToken           string `json:"access_token"`
	TokenType             string `json:"token_type"`
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
	return nil
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

type EventStore interface {
	CreatePostEvent(mf mf2.MicroFormat) error
	FetchAll() ([]MfMutator, error)
}

type MfMutator interface {
	Apply(mf2.MicroFormat) mf2.MicroFormat
}

type nullEvent struct {
}

func (e nullEvent) Apply(mf mf2.MicroFormat) mf2.MicroFormat {
	return mf
}

type PostCreatedEvent struct {
	EventID      string          `json:"eventID"`
	EventType    string          `json:"eventType"`
	EventVersion string          `json:"eventVersion"`
	EventData    mf2.MicroFormat `json:"eventData"`
}

type eventStore struct {
	Uploader    *s3manager.Uploader
	Downloader  *s3manager.Downloader
	S3Client    *s3.S3
	S3KeyPrefix string
	S3Bucket    string
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

func NewDynamoSessionStore() (SessionStore, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	if err != nil {
		return dynamoSessionStore{}, err
	}
	svc := dynamodb.New(sess)
	return dynamoSessionStore{TableName: "sessions", Db: svc}, nil
}

func (store dynamoSessionStore) FetchByID(postID string) (UserSession, error) {
	s, err := store.getItem(postID, store.TableName)
	if err != nil {
		return s, err
	}
	return s, nil
}

func (store dynamoSessionStore) Create(usess UserSession) error {
	err := store.putItem(usess, store.TableName)
	if err != nil {
		return err
	}
	return nil
}

func (store dynamoSessionStore) getItem(id string, tableName string) (UserSession, error) {
	var s UserSession
	// TODO add specific error checks
	result, err := store.Db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"uid": {S: aws.String(id)},
		},
	},
	)
	if err != nil {
		return s, err
	}
	err = dynamodbattribute.UnmarshalMap(result.Item, &s)
	if err != nil {
		return s, err
	}
	// TODO check for no item (no Item property)
	return s, nil
}

func (store dynamoSessionStore) putItem(in interface{}, tableName string) error {
	log.Printf("putItem: %+v", in)
	av, err := dynamodbattribute.MarshalMap(in)
	if err != nil {
		return err
	}
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}
	_, err = store.Db.PutItem(input)
	if err != nil {
		return err
	}
	return nil
}

type s3PostStore struct {
	prefix     string
	bucket     string
	basePath   string
	uploader   *s3manager.Uploader
	downloader *s3manager.Downloader
}

func NewS3PostStore(p, b, basePath string) (PostStore, error) {
	s, err := session.NewSession()
	if err != nil {
		return s3PostStore{}, err
	}
	uploader := s3manager.NewUploader(s)
	downloader := s3manager.NewDownloader(s)

	return s3PostStore{
		prefix:     p,
		bucket:     b,
		uploader:   uploader,
		downloader: downloader,
		basePath:   basePath,
	}, nil
}

func (store s3PostStore) CreatePost(mf mf2.MicroFormat) error {

	// json encode
	//TODO extract this func
	mfjson := new(bytes.Buffer)
	err := json.NewEncoder(mfjson).Encode(mf.ToView().PrepForHugo())
	if err != nil {
		return fmt.Errorf("failed to mf encode json %v", err)
	}

	contentPath := "/content/p/"
	dataPath := "/data/"
	fn := mf.ToView().Uid

	err = ioutil.WriteFile(store.basePath+contentPath+fn+".md", mfjson.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %s", err.Error())
	}
	err = ioutil.WriteFile(store.basePath+dataPath+fn+".json", mfjson.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %s", err.Error())
	}

	//log.Printf("%s", mf.ToView().Url)

	return nil
}

func (ps s3PostStore) putItem(k string, data *bytes.Buffer) error {
	_, err := ps.uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(ps.bucket),
		Key:    aws.String(k),
		Body:   data,
		ACL:    aws.String("private"),
	})
	if err != nil {
		return err
	}
	return nil
}

func (ps s3PostStore) getItem(k string) (*bytes.Buffer, error) {
	in := s3.GetObjectInput{
		Bucket: aws.String(ps.bucket),
		Key:    aws.String(k),
	}
	buf := aws.NewWriteAtBuffer([]byte{})
	_, err := ps.downloader.Download(buf, &in)
	if err != nil {
		return &bytes.Buffer{}, err
	}
	return bytes.NewBuffer(buf.Bytes()), nil
}

func (ps s3PostStore) FetchByID(postID string) (mf2.MicroFormatView, error) {
	var jf2 mf2.MicroFormatView
	var mf mf2.MicroFormat

	fileKey := strings.Trim(ps.prefix, "/ ") + "/posts/" + postID + ".json"

	buf, err := ps.getItem(fileKey)
	if err != nil {
		return jf2, err
	}
	err = json.Unmarshal(buf.Bytes(), &mf)
	if err != nil {
		return jf2, err
	}

	return mf.ToView(), nil
}

func (store s3PostStore) FetchFeed(feedID string, limit int) (mf2.MicroFormatView, error) {
	var mf mf2.MicroFormatView
	return mf, nil
}

func NewDynamoPostStore(tableName, feedTableName string) (PostStore, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	if err != nil {
		return dynamoPostStore{}, err
	}
	svc := dynamodb.New(sess)
	return dynamoPostStore{TableName: tableName, Db: svc, FeedTableName: feedTableName}, nil
}

func (store dynamoPostStore) CreatePost(mf mf2.MicroFormat) error {
	err := store.putItem(mf.ToView(), store.TableName)
	if err != nil {
		return err
	}

	for _, feedID := range mf.Feeds() {
		feed, err := store.getItem(feedID, store.FeedTableName)
		if err != nil {
			return err
		}
		feed.Uid = feedID
		feed.Children = append(feed.Children, mf.ToView())
		err = store.putItem(feed, store.FeedTableName)
		if err != nil {
			return err
		}
	}

	return nil
}

func (store dynamoPostStore) putItem(in interface{}, tableName string) error {
	av, err := dynamodbattribute.MarshalMap(in)
	if err != nil {
		return err
	}
	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}
	_, err = store.Db.PutItem(input)
	if err != nil {
		return err
	}
	return nil
}

func (store dynamoPostStore) FetchByID(postID string) (mf2.MicroFormatView, error) {
	jf2, err := store.getItem(postID, store.TableName)
	if err != nil {
		return jf2, err
	}
	return jf2, nil
}

func (store dynamoPostStore) getItem(id string, tableName string) (mf2.MicroFormatView, error) {
	var jf2 mf2.MicroFormatView
	result, err := store.Db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"uid": {S: aws.String(id)},
		},
	},
	)
	if err != nil {
		return jf2, err
	}
	err = dynamodbattribute.UnmarshalMap(result.Item, &jf2)
	if err != nil {
		return jf2, err
	}
	return jf2, nil
}

func (store dynamoPostStore) FetchFeed(feedID string, limit int) (mf2.MicroFormatView, error) {
	jf2, err := store.getItem(feedID, store.FeedTableName)
	if err != nil {
		return jf2, err
	}
	return jf2, nil
}

func New(DB_FILE string) (PostStore, error) {
	// connect to db
	db, err := sql.Open("sqlite3", DB_FILE)
	if err != nil {
		return sqliteStore{}, err
	}
	store := sqliteStore{Db: db}
	err = store.Init()
	if err != nil {
		return sqliteStore{}, err
	}
	return store, nil
}

func (store sqliteStore) Init() error {
	// init db
	q := "CREATE TABLE IF NOT EXISTS posts (id PRIMARY KEY, post)"
	stmt, err := store.Db.Prepare(q)
	if err != nil {
		return err
	}
	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	q = "CREATE TABLE IF NOT EXISTS feeds (id PRIMARY KEY, feed)"
	stmt, err = store.Db.Prepare(q)
	if err != nil {
		return err
	}
	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	return nil
}

func (store sqliteStore) CreatePost(mf mf2.MicroFormat) error {
	jf2 := mf.ToView()
	jf2json := new(bytes.Buffer)
	err := json.NewEncoder(jf2json).Encode(jf2)
	if err != nil {
		return err
	}

	// save to sql
	q := "INSERT INTO posts (id, post) VALUES (?,?)"
	stmt, err := store.Db.Prepare(q)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(jf2.Uid, jf2json.String())
	if err != nil {
		return err
	}

	err = store.addPostToFeeds(mf)
	if err != nil {
		return err
	}

	return nil
}

func (store sqliteStore) addPostToFeeds(mf mf2.MicroFormat) error {

	for _, feedID := range mf.Feeds() {
		feed, err := store.FetchFeed(feedID, 0)
		if err != nil {
			return err
		}
		feed.Children = append(feed.Children, mf.ToView())
		err = store.saveFeed(feed)
		if err != nil {
			return err
		}
	}

	return nil
}

func (store sqliteStore) saveFeed(feed mf2.MicroFormatView) error {
	// json encode
	feedJson := new(bytes.Buffer)
	err := json.NewEncoder(feedJson).Encode(feed)
	if err != nil {
		log.Printf("failed to encode json %v", err)
		return err
	}
	log.Printf("feed: %v", feedJson)

	q := "REPLACE INTO feeds (id, feed) VALUES (?,?)"
	stmt, err := store.Db.Prepare(q)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(feed.Name, feedJson.String())
	if err != nil {
		return err
	}

	return nil
}

func (store sqliteStore) FetchFeed(feedID string, limit int) (mf2.MicroFormatView, error) {
	var jf2 = mf2.MicroFormatView{
		Name: feedID,
		Type: "feed",
	}
	var feedJson string

	rows, err := store.Db.Query("SELECT feed FROM feeds WHERE id = ?", feedID)
	if err != nil {
		return jf2, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&feedJson)
		if err != nil {
			return jf2, err
		}
		err = json.Unmarshal([]byte(feedJson), &jf2)
		if err != nil {
			return jf2, err
		}
		log.Printf("FetchFeed: %s", feedJson)
	}

	return jf2, nil
}

func (store sqliteStore) FetchByID(postID string) (mf2.MicroFormatView, error) {
	var jf2 mf2.MicroFormatView
	var postJson string

	rows, err := store.Db.Query("SELECT post FROM posts WHERE id = ?", postID)
	if err != nil {
		return jf2, err
	}
	defer rows.Close()
	rows.Next()
	err = rows.Scan(&postJson)
	if err != nil {
		return jf2, err
	}

	err = json.Unmarshal([]byte(postJson), &jf2)
	if err != nil {
		return jf2, err
	}

	return jf2, nil
}

func NewEventStore(bucket, prefix string) (EventStore, error) {
	s, err := session.NewSession()
	if err != nil {
		return eventStore{}, err
	}
	uploader := s3manager.NewUploader(s)
	downloader := s3manager.NewDownloader(s)
	s3client := s3.New(s)

	return eventStore{
		S3Client:    s3client,
		Uploader:    uploader,
		Downloader:  downloader,
		S3Bucket:    bucket,
		S3KeyPrefix: prefix,
	}, nil
}

func (mfh eventStore) CreatePostEvent(mf mf2.MicroFormat) error {
	event := newPostCreated(mf)

	// json encode
	eventjson := new(bytes.Buffer)
	err := json.NewEncoder(eventjson).Encode(event)
	if err != nil {
		log.Printf("failed to encode json %v", err)
		return err
	}
	log.Printf("event: %v", eventjson)

	fileKey := strings.Trim(mfh.S3KeyPrefix, "/ ") + "/" + time.Now().Format("2006/") + event.EventID + ".json"
	_, err = mfh.Uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(mfh.S3Bucket),
		Key:    aws.String(fileKey),
		Body:   eventjson,
		ACL:    aws.String("private"),
	})
	if err != nil {
		log.Printf("failed to upload to s3 %v", err)
		return err
	}
	log.Printf("uploaded event to %s", fileKey)

	return nil
}

func (es eventStore) FetchAll() ([]MfMutator, error) {
	var l []MfMutator

	allKeys, err := es.getAllKeys()
	if err != nil {
		return l, err
	}
	log.Printf("%d keys found", len(allKeys))

	var errors []error
	for _, k := range allKeys {
		buf, err := es.fetchEventByKey(k)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		event, err := es.unmarshalEvent(buf)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		l = append(l, event)
	}
	log.Printf("failed to unmarshal %d events: %v", len(errors), errors)

	return l, nil
}

func (es eventStore) fetchEventByKey(k *string) (*bytes.Buffer, error) {
	// read event data
	in := s3.GetObjectInput{
		Bucket: aws.String(es.S3Bucket),
		Key:    k,
	}
	buf := aws.NewWriteAtBuffer([]byte{})
	_, err := es.Downloader.Download(buf, &in)
	if err != nil {
		return &bytes.Buffer{}, err
	}
	return bytes.NewBuffer(buf.Bytes()), nil
}

func (es eventStore) unmarshalEvent(buf *bytes.Buffer) (MfMutator, error) {
	var nul nullEvent
	// determine event type
	type eventType struct {
		EventType string `json:"eventType"`
	}
	var eType eventType
	err := json.Unmarshal(buf.Bytes(), &eType)
	if err != nil {
		return nul, fmt.Errorf("failed to unmarshal event type: %s", err.Error())
	}

	if eType.EventType == "PostCreated" {
		var event PostCreatedEvent
		err = json.Unmarshal(buf.Bytes(), &event)
		if err != nil {
			return nul, fmt.Errorf("failed to unmarshal event: %s %s", buf.String(), err.Error())
		}
		return event, nil
	}

	return nul, fmt.Errorf("failed to recognise the eventType: %s", eType.EventType)
}

func (es eventStore) getAllKeys() ([]*string, error) {
	var l []*string

	i := 0
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(es.S3Bucket),
		Prefix: aws.String(es.S3KeyPrefix),
	}
	err := es.S3Client.ListObjectsV2Pages(
		input,
		func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, item := range page.Contents {
				l = append(l, item.Key)
				i += 1
				//TODO rm
				//if i > 5 {
				//return false
				//}
			}
			if lastPage == true {
				return false
			}
			return true
		},
	)
	if err != nil {
		return l, err
	}

	return l, nil
}

func newPostCreated(mf mf2.MicroFormat) PostCreatedEvent {
	uid, err := uuid.NewV4()
	if err != nil {
		log.Fatalf("failed to create uuid %v", err)
	}
	return PostCreatedEvent{
		EventID:      uid.String(),
		EventType:    "PostCreated",
		EventVersion: time.Now().Format("20060102150405.0000"),
		EventData:    mf}
}

func (e PostCreatedEvent) Apply(mf mf2.MicroFormat) mf2.MicroFormat {
	return e.EventData
}
