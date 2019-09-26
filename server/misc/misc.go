package misc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
