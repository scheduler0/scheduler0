package misc

import (
	"fmt"
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

	return psgc
}

func (writer LogWriter) Write(bytes []byte) (int, error) {
	return fmt.Print(time.Now().UTC().Format("2006-01-02 15:04:05") + " [DEBUG] " + string(bytes))
}
