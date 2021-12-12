package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func postParams(httpClient *http.Client, uri string, data map[string]string, target interface{}) error {
	var encoded string
	if data != nil {
		values := url.Values{}
		for key, val := range data {
			values.Set(key, val)
		}
		encoded = values.Encode()
	}
	r, err := httpClient.Post(uri, "application/x-www-form-urlencoded", strings.NewReader(encoded))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	b, _ := ioutil.ReadAll(r.Body)
	if target != nil {
		err = json.Unmarshal(b, target)
		if err != nil {
			if strings.Contains(string(b), " upgrade ") {
				log.Printf("The client version you are using is not accepted by the server")
				os.Exit(5)
			}
			log.Printf("Bad JSON from %s -- %s\n", uri, string(b))
		}
	}
	return err
}

// Creates a new file upload http request with optional extra params
func BuildUploadRequest(uri string, params map[string]string, paramName, path string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(paramName, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", uri, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, err
}

type NextGameResponse struct {
	Type         string
	TrainingId   uint
	NetworkId    uint
	Sha          string
	CandidateSha string
	Params       string
	Flip         bool
	MatchGameId  uint
	KeepTime     string
	BookUrl      string
	BookSha      string
}

func NextGame(httpClient *http.Client, hostname string, params map[string]string) (NextGameResponse, error) {
	resp := NextGameResponse{}
	err := postParams(httpClient, hostname+"/next_game", params, &resp)

	if len(resp.Sha) == 0 {
		return resp, errors.New("Server gave back empty SHA")
	}

	return resp, err
}

func UploadMatchResult(httpClient *http.Client, hostname string, match_game_id uint, result int, pgn string, params map[string]string) error {
	params["match_game_id"] = strconv.Itoa(int(match_game_id))
	params["result"] = strconv.Itoa(result)
	params["pgn"] = pgn
	return postParams(httpClient, hostname+"/match_result", params, nil)
}

func DownloadNetwork(httpClient *http.Client, uriPrefix string, networkPath string, sha string) error {
	uri := uriPrefix + sha
	r, err := httpClient.Get(uri)
	if err != nil {
		return err
	}

	if r.StatusCode >= 400 {
		return errors.New("Network server gave error status.")
	}

	dir, _ := filepath.Split(networkPath)
	out, err := ioutil.TempFile(dir, sha+"_tmp")
	if err != nil {
		return err
	}

	_, err = io.Copy(out, r.Body)
	r.Body.Close()
	out.Close()
	if err == nil {
		err = os.Rename(out.Name(), networkPath)
	}
	// Ensure tmpfile is erased
	os.Remove(out.Name())
	return err
}
