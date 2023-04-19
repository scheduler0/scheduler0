package db

import (
	"database/sql"
	"fmt"
	"github.com/hashicorp/go-hclog"
	_ "github.com/iamf-dev/scheduler0-sqlite"
	"github.com/spf13/afero"
	"io"
	"log"
	"os"
	"scheduler0/constants"
	"scheduler0/utils"
	"sync"
)

type DataStore struct {
	dbFilePath string
	FileLock   sync.Mutex

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
	db.FileLock.Lock()
	defer db.FileLock.Unlock()

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
	db.FileLock.Lock()
	defer db.FileLock.Unlock()

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

	fs := afero.NewOsFs()
	dirPath := fmt.Sprintf("%s/%s/%s", dir, constants.SqliteDir)
	filePath := fmt.Sprintf("%s/%s/%s", dir, constants.SqliteDir, constants.SqliteDbFileName)
	exists, err := afero.DirExists(fs, dirPath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error checking dir exist: %s \n", err))
	}

	if !exists {
		utils.RecreateDb()
		RunMigration(logger)
		utils.RecreateRaftDir()
	}

	sqliteDb := NewSqliteDbConnection(logger, filePath)
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
	timezone 	   TEXT NOT NULL,
	timezone_offset INTEGER NOT NULL,
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

func RunMigration(cmdLogger hclog.Logger) {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	fs := afero.NewOsFs()

	dbDirPath := fmt.Sprintf("%s/%s", dir, constants.SqliteDir)
	dbFilePath := fmt.Sprintf("%s/%s/%s", dir, constants.SqliteDir, constants.SqliteDbFileName)

	err = fs.Remove(dbFilePath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalln(fmt.Errorf("Fatal db delete error: %s \n", err))
	}

	err = fs.Mkdir(dbDirPath, os.ModePerm)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal db file creation error: %s \n", err))
	}

	_, err = fs.Create(dbFilePath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal db file creation error: %s \n", err))
	}

	datastore := NewSqliteDbConnection(cmdLogger, dbFilePath)
	conn := datastore.OpenConnection()

	dbConnection := conn.(*sql.DB)

	trx, dbConnErr := dbConnection.Begin()
	if dbConnErr != nil {
		log.Fatalln(fmt.Errorf("Fatal open db transaction error: %s \n", dbConnErr))
	}

	_, execErr := trx.Exec(GetSetupSQL())
	if execErr != nil {
		errRollback := trx.Rollback()
		if errRollback != nil {
			log.Fatalln(fmt.Errorf("Fatal rollback error: %s \n", execErr))
		}
		log.Fatalln(fmt.Errorf("Fatal open db transaction error: %s \n", execErr))
	}

	errCommit := trx.Commit()
	if errCommit != nil {
		log.Fatalln(fmt.Errorf("Fatal commit error: %s \n", errCommit))
	}
}
