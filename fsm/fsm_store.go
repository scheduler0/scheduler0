package fsm

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/raft"
	"io"
	"io/ioutil"
	"log"
	"os"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/marsher"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"scheduler0/utils"
	"sync"
)

type Store struct {
	rwMtx           sync.RWMutex
	SqliteDB        db.DataStore
	logger          *log.Logger
	SQLDbConnection *sql.DB
	Raft            *raft.Raft
	PendingJobs     chan models.JobModel

	raft.BatchingFSM
}

type Response struct {
	Data  []interface{}
	Error string
}

var _ raft.FSM = &Store{}

func NewFSMStore(db db.DataStore, sqlDbConnection *sql.DB, logger *log.Logger) *Store {
	return &Store{
		SqliteDB:        db,
		SQLDbConnection: sqlDbConnection,
		PendingJobs:     make(chan models.JobModel, 100),
		logger:          logger,
	}
}

func (s *Store) Apply(l *raft.Log) interface{} {
	s.rwMtx.Lock()
	defer s.rwMtx.Unlock()

	return ApplyCommand(l, s.SQLDbConnection, true, s.PendingJobs)
}

func (s *Store) ApplyBatch(logs []*raft.Log) []interface{} {
	s.rwMtx.Lock()
	defer s.rwMtx.Unlock()

	results := []interface{}{}

	for _, l := range logs {
		result := ApplyCommand(l, s.SQLDbConnection, true, s.PendingJobs)
		results = append(results, result)
	}

	return results
}

func ApplyCommand(l *raft.Log, SQLDbConnection *sql.DB, queueJobs bool, queue chan models.JobModel) interface{} {
	if l.Type == raft.LogConfiguration {
		return nil
	}

	command := &protobuffs.Command{}

	err := marsher.UnmarshalCommand(l.Data, command)
	if err != nil {
		log.Fatal("failed to unmarshal command", err.Error())
	}
	configs := utils.GetScheduler0Configurations()

	switch command.Type {
	case protobuffs.Command_Type(constants.CommandTypeDbExecute):
		params := []interface{}{}
		err := json.Unmarshal(command.Data, &params)
		if err != nil {
			return Response{
				Data:  nil,
				Error: err.Error(),
			}
		}
		exec, err := SQLDbConnection.Exec(command.Sql, params...)
		if err != nil {
			fmt.Println(err.Error())
			return Response{
				Data:  nil,
				Error: err.Error(),
			}
		}

		lastInsertedId, err := exec.LastInsertId()
		if err != nil {
			fmt.Println(err.Error())
			return Response{
				Data:  nil,
				Error: err.Error(),
			}
		}
		rowsAffected, err := exec.RowsAffected()
		if err != nil {
			fmt.Println(err.Error())
			return Response{
				Data:  nil,
				Error: err.Error(),
			}
		}
		data := []interface{}{lastInsertedId, rowsAffected}

		return Response{
			Data:  data,
			Error: "",
		}
	case protobuffs.Command_Type(constants.CommandTypeJobQueue):
		utils.Info("command.Sql ", command.Sql, " configs.RaftAddress", configs.RaftAddress, queueJobs)
		if command.Sql == configs.RaftAddress && queueJobs {
			job := models.JobModel{}
			err := json.Unmarshal(command.Data, &job)
			if err != nil {
				return Response{
					Data:  nil,
					Error: err.Error(),
				}
			}

			utils.Info("received job ", command.Data)
			queue <- job
		}
	}

	return nil
}

func (s *Store) Snapshot() (raft.FSMSnapshot, error) {
	fmt.Println("Snapshot::")
	fmsSnapshot := NewFSMSnapshot(s.SqliteDB)
	return fmsSnapshot, nil
}

func (s *Store) Restore(r io.ReadCloser) error {
	fmt.Println("Restore::")
	b, err := utils.BytesFromSnapshot(r)
	if err != nil {
		return fmt.Errorf("restore failed: %s", err.Error())
	}
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	dbFilePath := fmt.Sprintf("%v/%v", dir, constants.SqliteDbFileName)
	if err := os.Remove(dbFilePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if b != nil {
		if err := ioutil.WriteFile(dbFilePath, b, os.ModePerm); err != nil {
			return err
		}
	}

	db, err := sql.Open("sqlite3", dbFilePath)
	if err != nil {
		return fmt.Errorf("restore failed to create db: %v", err)
	}

	err = db.Ping()
	if err != nil {
		s.logger.Println()
		return fmt.Errorf("ping error: restore failed to create db: %v", err)
	}

	return nil
}