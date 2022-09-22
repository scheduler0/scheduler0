package db

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"log"
	"os"
	"scheduler0/constants"
	"sync"
)

type sqlLiteDb struct {
	dbFilePath string
	rwMux      sync.RWMutex
}

type DataStore interface {
	OpenConnection() (io.Closer, error)
	Serialize() []byte
}

func NewSqliteDbConnection(dbFilePath string) DataStore {
	return &sqlLiteDb{
		dbFilePath: dbFilePath,
	}
}

// OpenConnection opens a database connection with one pool
func (db *sqlLiteDb) OpenConnection() (io.Closer, error) {
	db.rwMux.Lock()
	defer db.rwMux.Unlock()

	dir, err := os.Getwd()
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	dbFilePath := fmt.Sprintf("%v/%v", dir, constants.SqliteDbFileName)

	return sql.Open("sqlite3", dbFilePath)
}

func (db *sqlLiteDb) Serialize() []byte {
	db.rwMux.Lock()
	defer db.rwMux.Unlock()

	dir, err := os.Getwd()
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	dbFilePath := fmt.Sprintf("%v/%v", dir, constants.SqliteDbFileName)
	data, err := os.ReadFile(dbFilePath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}

	return data
}

func NewMemSqliteDd() (io.Closer, error) {
	return sql.Open("sqlite3", ":memory:")
}
