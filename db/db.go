package db

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"log"
	"os"
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

	return sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1", db.dbFilePath))
}

func (db *sqlLiteDb) Serialize() []byte {
	db.rwMux.Lock()
	defer db.rwMux.Unlock()

	data, err := os.ReadFile(db.dbFilePath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}

	return data
}

func GetSetupSQL() string {
	return `
CREATE TABLE IF NOT EXISTS credentials
(
    id                               INTEGER PRIMARY KEY AUTOINCREMENT,
    archived                         boolean   NOT NULL,
    api_key                          TEXT,
    api_secret                       TEXT,
    date_created                     datetime NOT NULL
);

CREATE TABLE IF NOT EXISTS projects
(
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT      NOT NULL UNIQUE,
    description  TEXT      NOT NULL,
    date_created datetime NOT NULL
);

CREATE TABLE IF NOT EXISTS jobs
(
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id     INTEGER   NOT NULL,
    spec           TEXT      NOT NULL,
    data           TEXT,
    callback_url   TEXT      NOT NULL,
    execution_type TEXT      NOT NULL,
    date_created   datetime NOT NULL,
    FOREIGN KEY (project_id)
        REFERENCES projects (id)
        ON DELETE CASCADE
);
`
}
