package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"database/sql"
	"fmt"

	log "github.com/sirupsen/logrus"

	// register the sqlite3 driver
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// config
	mediaEndpoint := "https://jay.funabashi.co.uk/micropub/media"
	mediaDIR := flag.String("dir", "", "media directory")
	flag.Parse()

	// deps
	logger := log.New()
	logger.
		WithField("endpoint", mediaEndpoint).
		WithField("directory", *mediaDIR).
		Info("importing media")

	if *mediaDIR == "" {
		logger.
			Error("please provide a 'dir' to import from")
		return
	}

	db, err := OpenDB()
	if err != nil {
		logger.WithError(err).
			Error("failed to open db")
		return
	}

	err = filepath.Walk(
		*mediaDIR,
		func(filepath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// TODO uploadMedia(filepath, logger)
			ext := strings.ToLower(path.Ext(filepath))

			if ext == ".jpg" {

				// OPEN FILE
				f, err := os.Open(filepath)
				if err != nil {
					logger.WithError(err).
						WithField("file_path", filepath).
						Error("failed to open file")
					return err
				}
				defer f.Close()

				// READ FILE HASH
				hash := md5.New()
				if _, err := io.Copy(hash, f); err != nil {
					logger.WithError(err).
						Errorf("failed to read file hash: %s", filepath)
					return err
				}
				hashInBytes := hash.Sum(nil)[:16]
				fileHash := hex.EncodeToString(hashInBytes)

				// IF HASH EXISTS, RETURN
				row := db.QueryRow(
					"SELECT count(*) FROM media_uploaded WHERE file_hash = ?",
					fileHash,
				)
				var hashExists int
				err = row.Scan(&hashExists)
				if err != nil {
					logger.WithError(err).
						Error("Failed to query media_uploaded")
					return err
				}
				if hashExists > 0 {
					return nil
				}

				// REWIND FILE
				_, err = f.Seek(0, 0)
				if err != nil {
					logger.
						WithError(err).
						Error("Failed to rewind file")
					return err
				}

				// UPLOAD TO MEDIA ENDPOINT
				mediaURL, err := uploadFile(mediaEndpoint, filepath, f)
				if err != nil {
					logger.
						WithError(err).
						WithField("file_path", filepath).
						Error("failed to upload file")
					return err
				}

				// INSERT FILEHASH TO DB
				_, err = db.Exec(
					"INSERT INTO media_uploaded (file_hash) VALUES (?);",
					fileHash,
				)
				if err != nil {
					logger.
						WithError(err).
						Error("failed to insert to media_uploaded table")
					return err
				}

				logger.
					WithField("file_path", filepath).
					WithField("media_url", mediaURL).
					Info("media uploaded")
				return nil
			}

			return nil
		},
	)

	if err != nil {
		logger.WithError(err).
			Errorf("failed to walk media directory: %s", *mediaDIR)
		return
	}

}

func uploadFile(mediaEndpoint, filename string, fileToUpload io.Reader) (string, error) {

	validToken := "invalid_token"

	// copy file to multipart body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(part, fileToUpload)
	if err != nil {
		return "", err
	}

	err = writer.Close()
	if err != nil {
		return "", err
	}

	// create media-endpoint request
	req, err := http.NewRequest("POST", mediaEndpoint, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+validToken)

	// perform request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("media endpoint returned non 201 %d", resp.StatusCode)
	}

	return resp.Header.Get("Location"), nil
}

func createDB() string {
	return `
CREATE TABLE IF NOT EXISTS "media_uploaded" (
	"file_hash" TEXT PRIMARY KEY
);

`
}

func OpenDB() (*sql.DB, error) {
	var db *sql.DB
	var err error
	defer func() {
		if err != nil && db != nil {
			db.Close()
		}
	}()

	dbPath := "file:index.sql"

	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %v", err)
	}

	// ensure DB is provisioned
	_, err = db.Exec(createDB())
	if err != nil {
		return nil, fmt.Errorf("setting up database: %v", err)
	}

	return db, nil
}
