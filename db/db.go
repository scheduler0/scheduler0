package db

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"scheduler0/constants"
	"sync"
)

type DataStore struct {
	dbFilePath string
	fileLock   sync.Mutex

	ConnectionLock sync.Mutex
	Connection     *sql.DB
}

func NewSqliteDbConnection(dbFilePath string) *DataStore {
	return &DataStore{
		dbFilePath: dbFilePath,
	}
}

// OpenConnection opens a database connection with one pool
func (db *DataStore) OpenConnection() io.Closer {
	db.fileLock.Lock()
	defer db.fileLock.Unlock()

	if db.Connection != nil {
		return db.Connection
	}

	once := sync.Once{}

	once.Do(func() {
		connection, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1", db.dbFilePath))
		if err != nil {
			log.Fatalln("failed to open db")
		}

		db.Connection = connection
	})

	return db.Connection
}

func (db *DataStore) Serialize() []byte {
	db.fileLock.Lock()
	defer db.fileLock.Unlock()

	data, err := os.ReadFile(db.dbFilePath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}

	return data
}

func GetDBConnection(logger *log.Logger) *DataStore {
	dir, err := os.Getwd()
	if err != nil {
		logger.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	dbFilePath := fmt.Sprintf("%v/%v", dir, constants.SqliteDbFileName)

	sqliteDb := NewSqliteDbConnection(dbFilePath)
	conn := sqliteDb.OpenConnection()

	dbConnection := conn.(*sql.DB)
	err = dbConnection.Ping()
	if err != nil {
		logger.Fatalln(fmt.Errorf("ping error: restore failed to create db: %v", err))
	}

	return sqliteDb
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
    execution_type TEXT      NOT NULL DEFAULT "http",
    date_created   datetime NOT NULL,
    FOREIGN KEY (project_id)
        REFERENCES projects (id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS job_executions_committed
(
	id						INTEGER PRIMARY KEY AUTOINCREMENT,
	unique_id 				TEXT,
	state					INTEGER NOT NULL,
	node_id					INTEGER NOT NULL,
	last_execution_time   	datetime NOT NULL,
	next_execution_time   	datetime NOT NULL,
	job_id					INTEGER NOT NULL,
    date_created   			datetime NOT NULL,
	job_queue_version 		INTEGER NOT NULL,
	execution_version 		INTEGER NOT NULL,
    FOREIGN KEY (job_id)
        REFERENCES jobs (id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS job_executions_uncommitted
(
	id						INTEGER PRIMARY KEY AUTOINCREMENT,
	unique_id 				TEXT,
	state					INTEGER NOT NULL,
	node_id					INTEGER NOT NULL,
	last_execution_time   	datetime NOT NULL,
	next_execution_time   	datetime NOT NULL,
	job_id					INTEGER NOT NULL,
    date_created   			datetime NOT NULL,
	job_queue_version 		INTEGER NOT NULL,
	execution_version 		INTEGER NOT NULL,
    FOREIGN KEY (job_id)
        REFERENCES jobs (id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS job_queues
(
	id						INTEGER PRIMARY KEY AUTOINCREMENT,
	node_id					INTEGER NOT NULL,
	lower_bound_job_id		INTEGER NOT NULL,
	upper_bound_job_id		INTEGER NOT NULL,
	version 				INTEGER NOT NULL,
    date_created  		 	datetime NOT NULL
);

CREATE TABLE IF NOT EXISTS job_queue_versions
(
	id						INTEGER PRIMARY KEY AUTOINCREMENT,
	version 				INTEGER NOT NULL,
	number_of_active_nodes  INTEGER NOT NULL
);
`
}
