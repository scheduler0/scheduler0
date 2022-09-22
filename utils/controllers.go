package utils

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// Response response returned by http server
type Response struct {
	Data    interface{} `json:"data"`
	Success bool        `json:"success"`
}

// ToJSON converts response to JSON
func (r *Response) ToJSON() []byte {
	data, err := json.Marshal(r)
	if err != nil {
		log.Fatalln(err)
	}
	return data
}

// SendJSON returns JSON response to client
func SendJSON(w http.ResponseWriter, data interface{}, success bool, status int, headers map[string]string) {
	resObj := Response{Data: data, Success: success}

	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Content-Type", "application/json")

	if headers != nil {
		for header, val := range headers {
			w.Header().Add(header, val)
		}
	}

	w.WriteHeader(status)
	if status != http.StatusNoContent {
		_, err := w.Write(resObj.ToJSON())
		CheckErr(err)
	}
}

// ValidateQueryString check is a query string is included in the request
func ValidateQueryString(queryString string, r *http.Request) (string, error) {
	param := r.URL.Query()[queryString]

	if param == nil || len(param[0]) < 1 {
		return "", errors.New(queryString + " is not provided")
	}

	return param[0], nil
}

// ExtractBody validates and extracts request body
func ExtractBody(w http.ResponseWriter, r *http.Request) []byte {
	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		SendJSON(w, "request body required", false, http.StatusUnprocessableEntity, nil)
		return nil
	}

	if len(body) < 1 {
		SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return nil
	}

	return body
}

// RetryOnError retries callback function
func RetryOnError(callback func() error, maxRetry int, delay int) error {
	lastKnowError := callback()
	numberOfRetriesLeft := maxRetry
	if lastKnowError != nil {
		for numberOfRetriesLeft > 0 {
			time.Sleep(time.Second * time.Duration(delay))
			lastKnowError = callback()
			if lastKnowError != nil {
				numberOfRetriesLeft--
			} else {
				break
			}
		}
	}

	return lastKnowError
}
