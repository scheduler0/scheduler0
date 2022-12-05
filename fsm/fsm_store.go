package fsm

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/raft"
	"io"
	"io/ioutil"
	"log"
	"os"
	"scheduler0/config"
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
	PendingJobs     chan []int64
	PrepareJobs     chan models.JobStateLog
	CommitJobs      chan models.JobStateLog
	ErrorJobs       chan models.JobStateLog
	StopAllJobs     chan bool

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
		PendingJobs:     make(chan []int64, 1),
		PrepareJobs:     make(chan models.JobStateLog, 1),
		CommitJobs:      make(chan models.JobStateLog, 1),
		StopAllJobs:     make(chan bool, 1),
		logger:          logger,
	}
}

func (s *Store) Apply(l *raft.Log) interface{} {
	s.rwMtx.Lock()
	defer s.rwMtx.Unlock()

	return ApplyCommand(
		s.logger,
		l,
		s.SQLDbConnection,
		true,
		s.PendingJobs,
		s.PrepareJobs,
		s.CommitJobs,
		s.ErrorJobs,
		s.StopAllJobs,
	)
}

func (s *Store) ApplyBatch(logs []*raft.Log) []interface{} {
	s.rwMtx.Lock()
	defer s.rwMtx.Unlock()

	results := []interface{}{}

	for _, l := range logs {
		result := ApplyCommand(
			s.logger,
			l,
			s.SQLDbConnection,
			true,
			s.PendingJobs,
			s.PrepareJobs,
			s.CommitJobs,
			s.ErrorJobs,
			s.StopAllJobs,
		)
		results = append(results, result)
	}

	return results
}

func ApplyCommand(
	logger *log.Logger,
	l *raft.Log,
	SQLDbConnection *sql.DB,
	useQueues bool,
	queue chan []int64,
	prepareQueue chan models.JobStateLog,
	commitQueue chan models.JobStateLog,
	errorQueue chan models.JobStateLog,
	stopAllJobsQueue chan bool) interface{} {

	logPrefix := logger.Prefix()
	logger.SetPrefix(fmt.Sprintf("%s[apply-raft-command] ", logPrefix))
	defer logger.SetPrefix(logPrefix)

	if l.Type == raft.LogConfiguration {
		return nil
	}

	command := &protobuffs.Command{}

	marsherErr := marsher.UnmarshalCommand(l.Data, command)
	if marsherErr != nil {
		logger.Fatal("failed to unmarshal command", marsherErr.Error())
	}
	configs := config.Configurations(logger)

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
		ctx := context.Background()
		tx, err := SQLDbConnection.BeginTx(ctx, nil)
		if err != nil {
			logger.Println("failed to execute sql command", err.Error())
			return Response{
				Data:  nil,
				Error: err.Error(),
			}
		}

		if utils.MonitorMemoryUsage(logger) {
			return Response{
				Data:  nil,
				Error: "out of memory",
			}
		}

		exec, err := tx.Exec(command.Sql, params...)
		if err != nil {
			logger.Println("failed to execute sql command", err.Error())
			err := tx.Rollback()
			if err != nil {
				return Response{
					Data:  nil,
					Error: err.Error(),
				}
			}
			return Response{
				Data:  nil,
				Error: err.Error(),
			}
		}

		err = tx.Commit()
		if err != nil {
			return Response{
				Data:  nil,
				Error: err.Error(),
			}
		}

		lastInsertedId, err := exec.LastInsertId()
		if err != nil {
			logger.Println("failed to get last ", err.Error())
			rollBackErr := tx.Rollback()
			if rollBackErr != nil {
				return Response{
					Data:  nil,
					Error: rollBackErr.Error(),
				}
			}
			return Response{
				Data:  nil,
				Error: err.Error(),
			}
		}
		rowsAffected, err := exec.RowsAffected()
		if err != nil {
			logger.Println(err.Error())
			rollBackErr := tx.Rollback()
			if rollBackErr != nil {
				return Response{
					Data:  nil,
					Error: rollBackErr.Error(),
				}
			}
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
		if command.Sql == configs.RaftAddress && useQueues {
			jobIds := []interface{}{}
			err := json.Unmarshal(command.Data, &jobIds)
			if err != nil {
				return Response{
					Data:  nil,
					Error: err.Error(),
				}
			}
			lowerBound := jobIds[0].(float64)
			upperBound := jobIds[1].(float64)
			logger.Println(fmt.Sprintf("received  jobs %v to %v to queue", lowerBound, upperBound))
			queue <- []int64{int64(lowerBound), int64(upperBound)}
		}
		break
	case protobuffs.Command_Type(constants.CommandTypePrepareJobExecutions):
		if useQueues {
			jobState := []models.JobStateLog{}
			err := json.Unmarshal(command.Data, &jobState)
			if err != nil {
				return Response{
					Data:  nil,
					Error: err.Error(),
				}
			}

			logger.Println(fmt.Sprintf("received %v jobs from %s to log prepare", len(jobState[0].Data), jobState[0].ServerAddress))
			prepareQueue <- jobState[0]
		}
		break
	case protobuffs.Command_Type(constants.CommandTypeCommitJobExecutions):
		if useQueues {
			jobState := []models.JobStateLog{}
			err := json.Unmarshal(command.Data, &jobState)
			if err != nil {
				return Response{
					Data:  nil,
					Error: err.Error(),
				}
			}

			logger.Println(fmt.Sprintf("received %v jobs from %s to log commit", len(jobState[0].Data), jobState[0].ServerAddress))
			commitQueue <- jobState[0]
		}
		break
	case protobuffs.Command_Type(constants.CommandTypeErrorJobExecutions):
		if useQueues {
			jobState := []models.JobStateLog{}
			err := json.Unmarshal(command.Data, &jobState)
			if err != nil {
				return Response{
					Data:  nil,
					Error: err.Error(),
				}
			}

			logger.Println(fmt.Sprintf("received %v jobs from %s to log prepare", len(jobState[0].Data), jobState[0].ServerAddress))
			errorQueue <- jobState[0]
		}
	case protobuffs.Command_Type(constants.CommandTypeStopJobs):
		if useQueues {
			stopAllJobsQueue <- true
		}
	}

	return nil
}

func (s *Store) Snapshot() (raft.FSMSnapshot, error) {
	logPrefix := s.logger.Prefix()
	s.logger.SetPrefix(fmt.Sprintf("%s[snapshot-fsm] ", logPrefix))
	defer s.logger.SetPrefix(logPrefix)
	fmsSnapshot := NewFSMSnapshot(s.SqliteDB)
	s.logger.Println("took snapshot")
	return fmsSnapshot, nil
}

func (s *Store) Restore(r io.ReadCloser) error {
	logPrefix := s.logger.Prefix()
	s.logger.SetPrefix(fmt.Sprintf("%s[restoring-snapshot] ", logPrefix))
	defer s.logger.SetPrefix(logPrefix)
	s.logger.Println("restoring snapshot")

	b, err := utils.BytesFromSnapshot(r)
	if err != nil {
		return fmt.Errorf("restore failed: %s", err.Error())
	}
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Fatal error getting working dir: %s \n", err)
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
		return fmt.Errorf("ping error: restore failed to create db: %v", err)
	}

	s.SQLDbConnection = db

	return nil
}
