package utils

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
)

type Response struct {
	Data    interface{} `json:"data"`
	Success bool        `json:"success"`
}

func (r *Response) ToJSON() []byte {
	data, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	return data
}

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


func ValidateQueryString(queryString string, r *http.Request) (string, error) {
	param := r.URL.Query()[queryString]

	if param == nil || len(param[0]) < 1 {
		return "", errors.New(queryString + " is not provided")
	}

	return param[0], nil
}

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
