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
	"scheduler0/server/db"
	"scheduler0/server/marsher"
	"scheduler0/server/protobuffs"
	"scheduler0/utils"
	"sync"
)

type Store struct {
	sMtx     sync.RWMutex
	SqliteDB db.DataStore

	SQLDbConnection *sql.DB
	dbConnMtx       sync.RWMutex

	rMtx sync.RWMutex
	Raft *raft.Raft
}

type Response struct {
	Data  []interface{}
	Error string
}

var _ raft.FSM = &Store{}

func NewFSMStore(db db.DataStore, sqlDbConnection *sql.DB) *Store {
	return &Store{
		SqliteDB:        db,
		SQLDbConnection: sqlDbConnection,
	}
}

func (s *Store) Apply(l *raft.Log) interface{} {
	s.sMtx.Lock()
	defer s.sMtx.Unlock()

	command := &protobuffs.Command{}

	err := marsher.UnmarshalCommand(l.Data, command)
	if err != nil {
		log.Fatal("failed to unmarshal command", err.Error())
	}

	lastIndex := s.Raft.LastIndex()

	if l.Index < lastIndex {
		return nil
	}

	switch command.Type {
	case protobuffs.Command_Type(constants.COMMAND_TYPE_DB_EXECUTE):
		params := []interface{}{}
		err := json.Unmarshal(command.Data, &params)
		if err != nil {
			return Response{
				Data:  nil,
				Error: err.Error(),
			}
		}
		s.dbConnMtx.Lock()
		defer s.dbConnMtx.Unlock()
		exec, err := s.SQLDbConnection.Exec(command.Sql, params...)
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
		log.Fatalln(fmt.Errorf("restore failed to create db: %v", err))
	}

	err = db.Ping()
	if err != nil {
		log.Fatalln(fmt.Errorf("ping error: restore failed to create db: %v", err))
	}

	return nil
}
