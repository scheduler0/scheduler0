package misc

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
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
	ENV_PROD Env = "ENV_PROD"
	ENV_TEST     = "ENV_TEST"
	ENV_DEV      = "ENV_DEV"
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
	if env == ENV_TEST {
		return &PostgresCredentials{
			Addr:     "localhost:5432",
			Password: "admin",
			Database: "scheduler0_test",
			User:     "admin",
		}
	}

	if env == ENV_DEV {
		return &PostgresCredentials{
			Addr:     "localhost:5432",
			Password: "admin",
			Database: "scheduler0_dev",
			User:     "admin",
		}
	}

	return &PostgresCredentials{
		Addr:  os.Getenv("POSTGRES_ADDRESS"),
		Password: os.Getenv("POSTGRES_USER"),
		Database: os.Getenv("POSTGRES_PASSWORD"),
		User:  os.Getenv("POSTGRES_DATABASE"),
	}
}

func (writer LogWriter) Write(bytes []byte) (int, error) {
	fmt.Println("-----------")
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

func GetRequestParam(r *http.Request, paramName string, paramPos int) (string, error) {
	params := mux.Vars(r)
	paths := strings.Split(r.URL.Path, "/")
	param := ""

	if len(params[paramName]) > 1 {
		param += params[paramName]
	}

	if len(param) < 1 && len(paths) > paramPos {
		param += paths[paramPos]
	}

	if len(param) < 1 {
		return "", errors.New("param does not exist")
	}

	return param, nil
}

// TODO: Make this return a map instead of a list
func GetRequestQueryString(query string) map[string]string {
	pairs := strings.Split(query, "&")
	params := make(map[string]string, len(pairs))

	for i := 0; i < len(pairs); i++ {
		ketValuePair := strings.Split(pairs[i], "=")
		params[ketValuePair[0]] = ketValuePair[1]
	}

	return params
}
