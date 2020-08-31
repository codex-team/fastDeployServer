package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
)

// MakeHTTPRequest - make HTTP request with specified method, body, URL and headers
func MakeHTTPRequest(method string, url string, body []byte, headers map[string]string) ([]byte, error) {
	client := &http.Client{}
	r := bytes.NewReader(body)
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// SendError - log error and send it via HTTP response with the code specified
func SendError(w http.ResponseWriter, error string, code int) {
	log.Printf("%s\n", error)
	http.Error(w, error, code)
	return
}
