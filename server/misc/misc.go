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

func GetPort() string {
	port := os.Getenv("PORT")

	if len(port) == 0 {
		port = "9090"
	}

	return ":" + port
}

func GetPostgresCredentials() *PostgresCredentials {
	psgc := &PostgresCredentials{
		Addr:     "localhost:5432",
		Password: "postgres",
		Database: "postgres",
		User:     "postgres",
	}

	addr := os.Getenv("POSTGRES_ADDRESS")
	user := os.Getenv("POSTGRES_USER")
	pass := os.Getenv("POSTGRES_PASSWORD")
	db := os.Getenv("POSTGRES_DATABASE")

	if len(addr) > 0 {
		psgc.Addr = addr
	}

	if len(pass) > 0 {
		psgc.Password = pass
	}

	if len(db) > 0 {
		psgc.Database = db
	}

	if len(user) > 0 {
		psgc.User = user
	}

	return psgc
}

func (writer LogWriter) Write(bytes []byte) (int, error) {
	return fmt.Print(time.Now().UTC().Format("2006-01-02 15:04:05") + " [DEBUG] " + string(bytes))
}

type Response struct {
	Data interface{} `json:"data"`
}

func (r *Response) ToJson() []byte {
	data, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	return data
}

func SendJson(w http.ResponseWriter, data interface{}, status int, headers map[string]string) {
	resObj := Response{Data: data}

	w.Header().Set("Access-Control-Allow-Origin", "*")
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

// TODO: Make this a middleware
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

func GetRequestQueryString(query string) [][]string {
	pairs := strings.Split(query, "&")
	params := make([][]string, len(pairs))
	x := 0

	for i := 0; i < len(pairs); i++ {
		kv := strings.Split(pairs[i], "=")
		if len(kv) == 2 {
			params[x] = []string{kv[0], kv[1]}
			x++
		}

		if len(kv) == 1 {
			params[x] = []string{kv[0], ""}
			x++
		}
	}

	return params
}
