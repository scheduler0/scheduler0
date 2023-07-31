package db

import (
	"github.com/hashicorp/go-hclog"
	"io/ioutil"
	"os"
	"scheduler0/utils"
	"testing"
)

func TestNewSqliteDbConnection(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())
	if sqliteDb == nil {
		t.Fatalf("Failed to create a new SQLite database connection")
	}
}

func TestOpenConnection(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := ioutil.TempFile("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())
	conn := sqliteDb.OpenConnectionToExistingDB()

	if conn == nil {
		t.Fatalf("Failed to open SQLite database connection")
	}
}

func TestSerialize(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := ioutil.TempFile("", "test-db.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration()
	sqliteDb.OpenConnectionToExistingDB()
	data := sqliteDb.Serialize()

	if len(data) == 0 {
		t.Fatalf("Failed to serialize SQLite database")
	}
}

func TestGetDBConnection(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sqliteDb := CreateConnectionFromNewDbIfNonExists(logger)

	if sqliteDb == nil {
		t.Fatalf("Failed to get SQLite database connection")
	}

	utils.RemoveSqliteDbDir()
}
