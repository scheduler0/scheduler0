package db

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"log"
	"os"
	"path/filepath"
	"scheduler0/db/migrate"
)

func OpenTestConnection() (io.Closer, error) {
	return sql.Open("sqlite3", filepath.Join(os.Getenv("GOPATH"), "/src/scheduler0/server/db/db_test.db"))
}

// GetTestDBConnection returns a pool of connection to the database for tests
func GetTestDBConnection() *sql.DB {
	conn, err := OpenTestConnection()

	if err != nil {
		log.Fatal(err)
	}

	return conn.(*sql.DB)
}

// TeardownTestDB is executed in tests to clear the database for stateless tests
func TeardownTestDB() {
	conn, err := OpenTestConnection()
	db := conn.(*sql.DB)
	truncateQuery := "" +
		"DELETE FROM credentials;" +
		"DELETE FROM executions;" +
		"DELETE FROM jobs;" +
		"DELETE FROM projects;"
	_, err = db.Exec(truncateQuery)
	if err != nil {
		fmt.Println("[ERROR]: Could not truncate tables: ", err.Error())
	}
}

// PrepareTestDB creates the tables, runs migrations and seeds
func PrepareTestDB() {
	migrator := migrate.NewMigrator("/db_test.db", filepath.Join(os.Getenv("GOPATH"), "/src/scheduler0/server/db"))
	migrator.OpenConnection()
	migrator.RunMigrations()
}
