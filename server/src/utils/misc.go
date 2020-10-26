package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

type PostgresCredentials struct {
	Addr     string
	User     string
	Password string
	Database string
}

type LogWriter struct{}

type Env string

const (
	EnvProd Env = "ENV_PROD"
	EnvTest     = "ENV_TEST"
	EnvDev      = "ENV_DEV"
)

func GetPort() string {
	port := os.Getenv("PORT")

	if len(port) == 0 {
		port = "9090"
	}

	return ":" + port
}

func GetAuthentication() (string, string) {
	usernameEnv := os.Getenv("username")
	passwordEnv := os.Getenv("password")

	username := "admin"
	password := "admin"

	if len(usernameEnv) == 0 {
		username += usernameEnv
	}

	if len(passwordEnv) == 0 {
		password += passwordEnv
	}

	return username, password
}

func GetClientHost() string {
	return os.Getenv("CLIENT_HOST")
}

func GetPostgresCredentials(env Env) *PostgresCredentials {
	return &PostgresCredentials{
		Addr:     os.Getenv("POSTGRES_ADDRESS"),
		Password: os.Getenv("POSTGRES_PASSWORD"),
		Database: os.Getenv("POSTGRES_DATABASE"),
		User:     os.Getenv("POSTGRES_USER"),
	}
}

func (writer LogWriter) Write(bytes []byte) (int, error) {
	fmt.Println("----------------------------------------------------------------")
	return fmt.Print(time.Now().UTC().Format(time.RFC1123) + " [DEBUG] " + string(bytes))
}

type Response struct {
	Data    interface{} `json:"data"`
	Success bool        `json:"success"`
}

func (r *Response) ToJson() []byte {
	data, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	return data
}

func SendJson(w http.ResponseWriter, data interface{}, success bool, status int, headers map[string]string) {
	resObj := Response{Data: data, Success: success}

	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Content-Type", "application/json")

	if headers != nil {
		for header, val := range headers {
			w.Header().Add(header, val)
		}
	}

	w.WriteHeader(status)
	_, err := w.Write(resObj.ToJson())
	CheckErr(err)
}

func CheckErr(e error) {
	if e != nil {
		panic(e)
	}
}

//
// TODO: Make this return a map instead of a list
func GetRequestQueryString(query string) map[string]string {
	pairs := strings.Split(query, "&")
	params := make(map[string]string, len(pairs))

	if len(query) < 1 {
		return params
	}

	for i := 0; i < len(pairs); i++ {
		ketValuePair := strings.Split(pairs[i], "=")
		params[ketValuePair[0]] = ketValuePair[1]
	}

	return params
}

func ValidateQueryString(queryString string, r *http.Request) (string, error) {
	param := r.URL.Query()[queryString]

	if param == nil || len(param[0]) < 1 {
		return "", errors.New(queryString + "is not provided")
	}

	return param[0], nil
}

func ExtractBody(w http.ResponseWriter, r *http.Request) []byte {
	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		SendJson(w, "request body required", false, http.StatusUnprocessableEntity, nil)
	}

	if len(body) < 1 {
		SendJson(w, "request body required", false, http.StatusBadRequest, nil)
	}

	return body
}