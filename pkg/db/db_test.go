package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"scheduler0-private/pkg/utils"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestNewSqliteDbConnection(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
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

	tempFile, err := os.CreateTemp("", "test-db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())
	conn := sqliteDb.OpenConnectionToExistingDB()

	if conn == nil {
		t.Fatalf("Failed to open SQLite database connection")
	}

	// Close the connection to clean up
	defer conn.Close()
}

func TestSerialize(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-db.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())
	sqliteDb.RunMigration(logger)
	conn := sqliteDb.OpenConnectionToExistingDB()
	defer conn.Close()
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

func TestConnectionLockUnlock(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-db")
	assert.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())
	conn := sqliteDb.OpenConnectionToExistingDB()
	defer conn.Close()

	// Test that ConnectionLock and ConnectionUnlock work
	// We can't directly test mutex behavior, but we can ensure they don't panic
	sqliteDb.ConnectionLock()
	sqliteDb.ConnectionUnlock()

	// Test that we can still use the connection after locking/unlocking
	dbConn := sqliteDb.GetOpenConnection()
	assert.NotNil(t, dbConn)
}

func TestFileLockUnlock(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-db")
	assert.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())
	conn := sqliteDb.OpenConnectionToExistingDB()
	defer conn.Close()

	// Test that FileLock and FileUnlock work
	sqliteDb.FileLock()
	sqliteDb.FileUnlock()

	// Test that we can still serialize after locking/unlocking
	data := sqliteDb.Serialize()
	assert.NotNil(t, data)
}

func TestGetOpenConnection(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-db")
	assert.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())

	// Before opening connection, should return nil
	conn := sqliteDb.GetOpenConnection()
	assert.Nil(t, conn)

	// After opening connection, should return the connection
	closer := sqliteDb.OpenConnectionToExistingDB()
	defer closer.Close()

	conn = sqliteDb.GetOpenConnection()
	assert.NotNil(t, conn)

	// Verify it's the same connection
	closerConn := closer.(*sql.DB)
	assert.Equal(t, conn, closerConn)
}

func TestUpdateOpenConnection(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-db")
	assert.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())
	originalConn := sqliteDb.OpenConnectionToExistingDB()
	defer originalConn.Close()

	// Create a new connection
	newConn, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1", tempFile.Name()))
	assert.NoError(t, err)
	defer newConn.Close()

	// Update the connection
	sqliteDb.UpdateOpenConnection(newConn)

	// Verify the connection was updated
	updatedConn := sqliteDb.GetOpenConnection()
	assert.Equal(t, newConn, updatedConn)
	assert.NotEqual(t, originalConn, updatedConn)
}

func TestBeginTx(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-db")
	assert.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())
	conn := sqliteDb.OpenConnectionToExistingDB()
	defer conn.Close()

	// Run migrations to ensure we have a valid schema
	sqliteDb.RunMigration(logger)

	// Test BeginTx with nil options
	ctx := context.Background()
	tx, err := sqliteDb.BeginTx(ctx, nil)
	assert.NoError(t, err)
	assert.NotNil(t, tx)

	// Test that we can execute a query in the transaction
	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY)")
	assert.NoError(t, err)

	// Commit the transaction
	err = tx.Commit()
	assert.NoError(t, err)

	// Test BeginTx with options
	txOptions := &sql.TxOptions{
		Isolation: sql.LevelDefault,
		ReadOnly:  false,
	}
	tx2, err := sqliteDb.BeginTx(ctx, txOptions)
	assert.NoError(t, err)
	assert.NotNil(t, tx2)

	err = tx2.Rollback()
	assert.NoError(t, err)
}

func TestOpenConnectionToExistingDB_ReturnsSameConnection(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-db")
	assert.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())

	// First call
	conn1 := sqliteDb.OpenConnectionToExistingDB()
	defer conn1.Close()

	// Second call should return the same connection
	conn2 := sqliteDb.OpenConnectionToExistingDB()

	// Verify they are the same
	assert.Equal(t, conn1, conn2)
}

func TestGetDBMEMConnection(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sqliteDb := GetDBMEMConnection(logger)
	assert.NotNil(t, sqliteDb)

	conn := sqliteDb.OpenConnectionToExistingDB()
	assert.NotNil(t, conn)
	defer conn.Close()

	// Test that we can use the in-memory database
	dbConn := conn.(*sql.DB)
	err := dbConn.Ping()
	assert.NoError(t, err)

	// Test that we can create a table and query it
	_, err = dbConn.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	assert.NoError(t, err)

	_, err = dbConn.Exec("INSERT INTO test (name) VALUES (?)", "test")
	assert.NoError(t, err)

	var name string
	err = dbConn.QueryRow("SELECT name FROM test WHERE id = 1").Scan(&name)
	assert.NoError(t, err)
	assert.Equal(t, "test", name)
}

func TestCreateConnectionFromNewDbIfNonExistsForNode(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	nodeId := uint64(999)
	sqliteDb := CreateConnectionFromNewDbIfNonExistsForNode(logger, nodeId)
	assert.NotNil(t, sqliteDb)

	conn := sqliteDb.OpenConnectionToExistingDB()
	assert.NotNil(t, conn)
	defer conn.Close()

	dbConn := conn.(*sql.DB)
	err := dbConn.Ping()
	assert.NoError(t, err)

	// Clean up - manually remove the node-specific directory
	dirPath, _ := utils.GetSqliteDbDirAndDbFilePathForNode(nodeId)
	defer os.RemoveAll(dirPath)
}

func TestSerialize_NonExistentFile(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a connection with a non-existent file path
	sqliteDb := NewSqliteDbConnection(logger, "/nonexistent/path/to/db.db")

	// Serialize should handle the error gracefully
	// When the file doesn't exist, os.ReadFile returns nil data and an error
	// The method logs the error and returns nil
	data := sqliteDb.Serialize()
	// Should return nil when file doesn't exist (os.ReadFile behavior)
	assert.Nil(t, data, "Serialize should return nil when file doesn't exist")
}

func TestRunMigration(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-db.db")
	assert.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())
	conn := sqliteDb.OpenConnectionToExistingDB()
	defer conn.Close()

	// Run migrations
	sqliteDb.RunMigration(logger)

	// Verify migrations table was created
	dbConn := conn.(*sql.DB)
	var count int
	err = dbConn.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&count)
	assert.NoError(t, err)
	assert.Greater(t, count, 0, "Migrations table should have at least one migration")

	// Run migrations again - should be idempotent
	sqliteDb.RunMigration(logger)

	// Count should remain the same (no duplicate migrations)
	var count2 int
	err = dbConn.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&count2)
	assert.NoError(t, err)
	assert.Equal(t, count, count2, "Running migrations twice should not create duplicates")
}

// TestRunMigration_BackfillsAICreditTablesOnExistingDB reproduces a node whose
// database predates 32032e7: "main_tables" is already recorded, so the
// account_ai_credits tables that commit appended to it were never created.
// The dedicated ai_credit_tables migration must fill that gap.
func TestRunMigration_BackfillsAICreditTablesOnExistingDB(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-db.db")
	assert.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())
	conn := sqliteDb.OpenConnectionToExistingDB()
	defer conn.Close()
	dbConn := conn.(*sql.DB)

	// Build the "old" database: run everything, then drop the credit tables and
	// forget the migration that owns them, leaving main_tables recorded.
	sqliteDb.RunMigration(logger)
	_, err = dbConn.Exec(`
		DROP TABLE account_ai_credit_ledger;
		DROP TABLE account_ai_credits;
		DELETE FROM migrations WHERE name = 'ai_credit_tables';
	`)
	assert.NoError(t, err)

	tableCount := func() int {
		var n int
		err := dbConn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('account_ai_credits', 'account_ai_credit_ledger')`).Scan(&n)
		assert.NoError(t, err)
		return n
	}
	assert.Equal(t, 0, tableCount(), "precondition: credit tables absent")

	var mainRecorded int
	err = dbConn.QueryRow(`SELECT COUNT(*) FROM migrations WHERE name = 'main_tables'`).Scan(&mainRecorded)
	assert.NoError(t, err)
	assert.Equal(t, 1, mainRecorded, "precondition: main_tables already applied")

	// Booting the node again must create the missing tables.
	sqliteDb.RunMigration(logger)
	assert.Equal(t, 2, tableCount(), "ai_credit_tables must create both tables on an existing DB")

	var idx int
	err = dbConn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name IN ('idx_account_ai_credit_ledger_idem', 'idx_account_ai_credit_ledger_account_date')`).Scan(&idx)
	assert.NoError(t, err)
	assert.Equal(t, 2, idx, "ledger indexes must be created")

	// And be a no-op when run once more.
	sqliteDb.RunMigration(logger)
	var recorded int
	err = dbConn.QueryRow(`SELECT COUNT(*) FROM migrations WHERE name = 'ai_credit_tables'`).Scan(&recorded)
	assert.NoError(t, err)
	assert.Equal(t, 1, recorded)
}

func TestConcurrentOpenConnection(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-db")
	assert.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())

	// Test concurrent access
	var wg sync.WaitGroup
	connections := make([]io.Closer, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn := sqliteDb.OpenConnectionToExistingDB()
			connections[idx] = conn
		}(i)
	}

	wg.Wait()

	// All connections should be the same instance
	firstConn := connections[0]
	for i := 1; i < 10; i++ {
		assert.Equal(t, firstConn, connections[i], "All concurrent calls should return the same connection")
	}

	// Clean up
	for _, conn := range connections {
		if conn != nil {
			conn.Close()
		}
	}
}

func TestConnectionLock_Concurrency(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "db-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	tempFile, err := os.CreateTemp("", "test-db")
	assert.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	sqliteDb := NewSqliteDbConnection(logger, tempFile.Name())
	conn := sqliteDb.OpenConnectionToExistingDB()
	defer conn.Close()

	// Test that ConnectionLock actually serializes access
	var wg sync.WaitGroup
	counter := 0
	mu := sync.Mutex{}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sqliteDb.ConnectionLock()
			defer sqliteDb.ConnectionUnlock()

			// Simulate some work
			mu.Lock()
			counter++
			current := counter
			mu.Unlock()

			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			assert.Equal(t, current, counter, "Counter should not change while locked")
			mu.Unlock()
		}()
	}

	wg.Wait()
	assert.Equal(t, 10, counter)
}
