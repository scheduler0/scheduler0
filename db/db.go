package db

import (
	"database/sql"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"io"
	"os"
	"scheduler0/constants"
	"sync"
)

type DataStore struct {
	dbFilePath string
	fileLock   sync.Mutex

	ConnectionLock sync.Mutex
	Connection     *sql.DB

	logger hclog.Logger
}

func NewSqliteDbConnection(logger hclog.Logger, dbFilePath string) *DataStore {
	return &DataStore{
		dbFilePath: dbFilePath,
		logger:     logger,
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
			db.logger.Error("failed to open db", err.Error())
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
		db.logger.Error("Fatal error getting working dir: %s \n", err)
	}

	return data
}

func GetDBConnection(logger hclog.Logger) *DataStore {
	dir, err := os.Getwd()
	if err != nil {
		logger.Error("Fatal error getting working dir: %s \n", err)
	}
	dbFilePath := fmt.Sprintf("%v/%v", dir, constants.SqliteDbFileName)

	sqliteDb := NewSqliteDbConnection(logger, dbFilePath)
	conn := sqliteDb.OpenConnection()

	dbConnection := conn.(*sql.DB)
	err = dbConnection.Ping()
	if err != nil {
		logger.Error("ping error: failed to create file db: %v", err)
	}

	return sqliteDb
}

func GetDBMEMConnection(logger hclog.Logger) *DataStore {
	conn, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1", ":memory:"))
	if err != nil {
		logger.Error("ping error: failed to create in memory db: %v", err)
	}
	return &DataStore{
		Connection: conn,
	}
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

CREATE TABLE IF NOT EXISTS async_tasks_committed
(
	id						INTEGER PRIMARY KEY AUTOINCREMENT,
	request_id 				TEXT,
	input  					TEXT NOT NULL,
	output  				TEXT,
	state					INTEGER NOT NULL,
	service					TEXT,
    date_created  		 	datetime NOT NULL
);

CREATE TABLE IF NOT EXISTS async_tasks_uncommitted
(
	id						INTEGER PRIMARY KEY AUTOINCREMENT,
	request_id 				TEXT,
	input  					TEXT NOT NULL,
	output  				TEXT,
	state					INTEGER NOT NULL,
	service					TEXT,
    date_created  		 	datetime NOT NULL
);
`
}
