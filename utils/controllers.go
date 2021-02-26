package utils

import (
	"encoding/json"
	"errors"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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
		panic(err)
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
	}

	if len(body) < 1 {
		SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
	}

	return body
}

// ExtractResponse extract response from response writer as string
func ExtractResponse(w *httptest.ResponseRecorder) (*utils.Response, string, error) {
	body, err := ioutil.ReadAll(w.Body)

	if err != nil {
		return nil, "", err
	}

	res := &utils.Response{}

	err = json.Unmarshal(body, res)
	if err != nil {
		return nil, "", err
	}

	return res, string(body), nil
}

