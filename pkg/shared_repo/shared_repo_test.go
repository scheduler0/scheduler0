package shared_repo

import (
	"context"
	"database/sql"
	"fmt"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/db"
	"scheduler0/pkg/db/migrations"
	"scheduler0/pkg/models"
	"slices"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

// runMigrationsOnConnection runs migrations directly on a database connection
func runMigrationsOnConnection(conn *sql.DB, logger hclog.Logger) {
	// Create migrations table
	_, err := conn.Exec("CREATE TABLE IF NOT EXISTS migrations (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)")
	if err != nil {
		logger.Error("Failed to create migrations table", "error", err)
		return
	}

	// Query existing migrations
	rows, err := conn.Query("SELECT name FROM migrations ORDER BY id")
	if err != nil {
		logger.Error("Failed to query migrations", "error", err)
		return
	}

	var existingMigrations []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			logger.Error("Failed to scan migration name", "error", err)
			rows.Close()
			return
		}
		existingMigrations = append(existingMigrations, name)
	}
	rows.Close()

	// Get all migrations
	allMigrations := []models.Migration{
		migrations.MainTables(),
		migrations.SeedAccountFeatures(),
		migrations.SeedSystemAccount(),
	}

	// Start transaction
	tx, err := conn.BeginTx(context.Background(), nil)
	if err != nil {
		logger.Error("Failed to begin transaction", "error", err)
		return
	}

	// Run migrations
	for _, migration := range allMigrations {
		if slices.Contains(existingMigrations, migration.Name) {
			continue
		}
		if err := migration.Up(tx); err != nil {
			logger.Error("Failed to run migration", "migration", migration.Name, "error", err)
			tx.Rollback()
			return
		}
		_, err := tx.Exec("INSERT INTO migrations (name) VALUES (?)", migration.Name)
		if err != nil {
			logger.Error("Failed to insert migration record", "migration", migration.Name, "error", err)
			tx.Rollback()
			return
		}
	}

	if err := tx.Commit(); err != nil {
		logger.Error("Failed to commit migrations", "error", err)
	}
}

func Test_GetUncommittedExecutionLogs_Returns_Uncommitted_Execution_Logs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "shard-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sharedRepo := NewSharedRepo(logger, scheduler0config)
	sqliteDb := db.GetDBMEMConnection(logger)
	conn := sqliteDb.GetOpenConnection()

	// Run migrations directly on in-memory database
	runMigrationsOnConnection(conn, logger)

	// Account with id=1 is created by seed_system_account migration, so we use id=2
	_, err := conn.Exec("INSERT INTO accounts (id, name, date_created) VALUES (2, 'Test Account', ?)", time.Now())
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	// Insert project directly
	_, err = conn.Exec("INSERT INTO projects (id, account_id, name, description, date_created) VALUES (1, 2, 'Test Project', 'Description', ?)", time.Now())
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Insert job directly
	_, err = conn.Exec("INSERT INTO jobs (id, account_id, project_id, spec, start_date, timezone, timezone_offset, retry_max, date_created, status, created_by) VALUES (1, 2, 1, '*/5 * * * *', ?, 'UTC', 0, 3, ?, 'active', 'test')", time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Create job execution log using sharedRepo
	nodeId := uint64(1)
	var uce models.JobExecutionLog
	gofakeit.Struct(&uce)
	uce.JobId = 1
	uce.NodeId = nodeId
	uce.State = models.ExecutionLogScheduleState
	uce.JobQueueVersion = 1
	uce.ExecutionVersion = 1
	uce.AccountId = 2
	now := time.Now()
	uce.DateCreated = now
	uce.DateModified = &now
	uce.LastExecutionDatetime = now
	uce.NextExecutionDatetime = now.Add(time.Hour)

	err = sharedRepo.InsertExecutionLogs(sqliteDb, false, []models.JobExecutionLog{uce})
	if err != nil {
		t.Fatalf("Failed to insert execution log: %v", err)
	}

	// Get execution logs
	foundUncommittedLogs, err := sharedRepo.GetExecutionLogs(sqliteDb, false)
	if err != nil {
		t.Fatalf("Failed to get uncommitted execution logs: %v", err)
	}

	assert.Equal(t, 1, len(foundUncommittedLogs))
	assert.Equal(t, uce.NodeId, foundUncommittedLogs[0].NodeId)
	assert.Equal(t, uce.UniqueId, foundUncommittedLogs[0].UniqueId)
	assert.Equal(t, uce.State, foundUncommittedLogs[0].State)
	assert.Equal(t, uce.JobId, foundUncommittedLogs[0].JobId)
	assert.Equal(t, uce.JobQueueVersion, foundUncommittedLogs[0].JobQueueVersion)
	assert.Equal(t, uce.ExecutionVersion, foundUncommittedLogs[0].ExecutionVersion)
	assert.Equal(t, uce.AccountId, foundUncommittedLogs[0].AccountId)
}

func Test_InsertToCommittedExecutionLogs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "shard-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sharedRepo := NewSharedRepo(logger, scheduler0config)
	sqliteDb := db.GetDBMEMConnection(logger)
	conn := sqliteDb.GetOpenConnection()

	// Run migrations directly on in-memory database
	runMigrationsOnConnection(conn, logger)

	// Account with id=1 is created by seed_system_account migration, so we use id=2
	_, err := conn.Exec("INSERT INTO accounts (id, name, date_created) VALUES (2, 'Test Account', ?)", time.Now())
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	// Insert project directly
	_, err = conn.Exec("INSERT INTO projects (id, account_id, name, description, date_created) VALUES (1, 2, 'Test Project', 'Description', ?)", time.Now())
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Insert job directly
	_, err = conn.Exec("INSERT INTO jobs (id, account_id, project_id, spec, start_date, timezone, timezone_offset, retry_max, date_created, status, created_by) VALUES (1, 2, 1, '*/5 * * * *', ?, 'UTC', 0, 3, ?, 'active', 'test')", time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	nodeId := uint64(3)
	numberOfJEL := 20

	var jobExecutionLogs []models.JobExecutionLog
	for i := 0; i < numberOfJEL; i++ {
		var uce models.JobExecutionLog
		gofakeit.Struct(&uce)
		uce.JobId = 1
		uce.NodeId = nodeId
		uce.State = models.ExecutionLogScheduleState
		uce.JobQueueVersion = 1
		uce.ExecutionVersion = uint64(i + 1)
		uce.AccountId = 2
		now := time.Now()
		uce.DateCreated = now
		uce.DateModified = &now
		uce.LastExecutionDatetime = now
		uce.NextExecutionDatetime = now.Add(time.Hour)
		jobExecutionLogs = append(jobExecutionLogs, uce)
	}

	err = sharedRepo.InsertExecutionLogs(sqliteDb, true, jobExecutionLogs)
	if err != nil {
		t.Fatalf("Failed to insert committed job execution logs: %v", err)
	}

	executionLogs, err := sharedRepo.GetExecutionLogs(sqliteDb, true)
	if err != nil {
		t.Fatalf("Failed to get committed execution logs: %v", err)
	}

	assert.Equal(t, numberOfJEL, len(executionLogs))
}

func Test_DeleteUncommittedExecutionLogs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "shard-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sharedRepo := NewSharedRepo(logger, scheduler0config)
	sqliteDb := db.GetDBMEMConnection(logger)
	conn := sqliteDb.GetOpenConnection()

	// Run migrations directly on in-memory database
	runMigrationsOnConnection(conn, logger)

	// Account with id=1 is created by seed_system_account migration, so we use id=2
	_, err := conn.Exec("INSERT INTO accounts (id, name, date_created) VALUES (2, 'Test Account', ?)", time.Now())
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	// Insert project directly
	_, err = conn.Exec("INSERT INTO projects (id, account_id, name, description, date_created) VALUES (1, 2, 'Test Project', 'Description', ?)", time.Now())
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Insert job directly
	_, err = conn.Exec("INSERT INTO jobs (id, account_id, project_id, spec, start_date, timezone, timezone_offset, retry_max, date_created, status, created_by) VALUES (1, 2, 1, '*/5 * * * *', ?, 'UTC', 0, 3, ?, 'active', 'test')", time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	nodeId := uint64(3)
	numberOfJEL := 20

	var jobExecutionLogs []models.JobExecutionLog
	for i := 0; i < numberOfJEL; i++ {
		var uce models.JobExecutionLog
		gofakeit.Struct(&uce)
		uce.JobId = 1
		uce.NodeId = nodeId
		uce.State = models.ExecutionLogScheduleState
		uce.JobQueueVersion = 1
		uce.ExecutionVersion = uint64(i + 1)
		uce.AccountId = 2
		now := time.Now()
		uce.DateCreated = now
		uce.DateModified = &now
		uce.LastExecutionDatetime = now
		uce.NextExecutionDatetime = now.Add(time.Hour)
		jobExecutionLogs = append(jobExecutionLogs, uce)
	}

	err = sharedRepo.InsertExecutionLogs(sqliteDb, false, jobExecutionLogs)
	if err != nil {
		t.Fatalf("Failed to insert uncommitted job execution logs: %v", err)
	}

	executionLogs, err := sharedRepo.GetExecutionLogs(sqliteDb, false)
	if err != nil {
		t.Fatalf("Failed to get uncommitted execution logs: %v", err)
	}

	err = sharedRepo.DeleteExecutionLogs(sqliteDb, false, executionLogs)
	if err != nil {
		t.Fatalf("Failed to delete uncommitted execution logs: %v", err)
	}

	executionLogs, err = sharedRepo.GetExecutionLogs(sqliteDb, false)
	if err != nil {
		t.Fatalf("Failed to get uncommitted execution logs: %v", err)
	}

	assert.Equal(t, 0, len(executionLogs))
}

func Test_InsertAsyncTasksLogs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "shard-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sharedRepo := NewSharedRepo(logger, scheduler0config)
	sqliteDb := db.GetDBMEMConnection(logger)
	conn := sqliteDb.GetOpenConnection()

	// Run migrations directly on in-memory database
	runMigrationsOnConnection(conn, logger)

	// Account with id=1 is created by seed_system_account migration, so we use id=2
	_, err := conn.Exec("INSERT INTO accounts (id, name, date_created) VALUES (2, 'Test Account', ?)", time.Now())
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	numberOfJEL := 20
	var asyncTasks []models.AsyncTask
	for i := 0; i < numberOfJEL; i++ {
		var task models.AsyncTask
		gofakeit.Struct(&task)
		task.AccountId = 2
		task.DateCreated = time.Now()
		task.DateModified = time.Now()
		asyncTasks = append(asyncTasks, task)
	}

	err = sharedRepo.InsertAsyncTasksLogs(sqliteDb, true, asyncTasks)
	if err != nil {
		t.Fatalf("Failed to insert async tasks: %v", err)
	}

	// Query async tasks directly from database
	table := constants.CommittedAsyncTableName
	rows, err := conn.Query(fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s FROM %s",
		constants.AsyncTasksRequestIdColumn,
		constants.AsyncTasksInputColumn,
		constants.AsyncTasksOutputColumn,
		constants.AsyncTasksStateColumn,
		constants.AsyncTasksServiceColumn,
		constants.AsyncTasksDateCreatedColumn,
		constants.AsyncTasksAccountIdColumn,
		constants.AsyncTasksDateModifiedColumn,
		table,
	))
	if err != nil {
		t.Fatalf("Failed to query async tasks: %v", err)
	}
	defer rows.Close()

	var ast []models.AsyncTask
	for rows.Next() {
		var task models.AsyncTask
		err := rows.Scan(
			&task.RequestId,
			&task.Input,
			&task.Output,
			&task.State,
			&task.Service,
			&task.DateCreated,
			&task.AccountId,
			&task.DateModified,
		)
		if err != nil {
			t.Fatalf("Failed to scan async task: %v", err)
		}
		ast = append(ast, task)
	}

	assert.Equal(t, len(asyncTasks), len(ast))
}

func Test_DeleteAsyncTasksLogs(t *testing.T) {
	scheduler0config := config.NewScheduler0Config()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "shard-repo-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	sharedRepo := NewSharedRepo(logger, scheduler0config)
	sqliteDb := db.GetDBMEMConnection(logger)
	conn := sqliteDb.GetOpenConnection()

	// Run migrations directly on in-memory database
	runMigrationsOnConnection(conn, logger)

	// Account with id=1 is created by seed_system_account migration, so we use id=2
	_, err := conn.Exec("INSERT INTO accounts (id, name, date_created) VALUES (2, 'Test Account', ?)", time.Now())
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	numberOfJEL := 20
	var asyncTasks []models.AsyncTask
	for i := 0; i < numberOfJEL; i++ {
		var task models.AsyncTask
		gofakeit.Struct(&task)
		task.AccountId = 2
		task.DateCreated = time.Now()
		task.DateModified = time.Now()
		asyncTasks = append(asyncTasks, task)
	}

	err = sharedRepo.InsertAsyncTasksLogs(sqliteDb, false, asyncTasks)
	if err != nil {
		t.Fatalf("Failed to insert async tasks: %v", err)
	}

	// Query async tasks directly from database
	table := constants.UnCommittedAsyncTableName
	rows, err := conn.Query(fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s FROM %s",
		constants.AsyncTasksRequestIdColumn,
		constants.AsyncTasksInputColumn,
		constants.AsyncTasksOutputColumn,
		constants.AsyncTasksStateColumn,
		constants.AsyncTasksServiceColumn,
		constants.AsyncTasksDateCreatedColumn,
		constants.AsyncTasksAccountIdColumn,
		constants.AsyncTasksDateModifiedColumn,
		table,
	))
	if err != nil {
		t.Fatalf("Failed to query async tasks: %v", err)
	}
	defer rows.Close()

	var ast []models.AsyncTask
	for rows.Next() {
		var task models.AsyncTask
		err := rows.Scan(
			&task.RequestId,
			&task.Input,
			&task.Output,
			&task.State,
			&task.Service,
			&task.DateCreated,
			&task.AccountId,
			&task.DateModified,
		)
		if err != nil {
			t.Fatalf("Failed to scan async task: %v", err)
		}
		ast = append(ast, task)
	}

	assert.Equal(t, len(asyncTasks), len(ast))

	err = sharedRepo.DeleteAsyncTasksLogs(sqliteDb, false, ast)
	if err != nil {
		t.Fatalf("Failed to delete async tasks: %v", err)
	}

	// Query again to verify deletion
	rows, err = conn.Query(fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s FROM %s",
		constants.AsyncTasksRequestIdColumn,
		constants.AsyncTasksInputColumn,
		constants.AsyncTasksOutputColumn,
		constants.AsyncTasksStateColumn,
		constants.AsyncTasksServiceColumn,
		constants.AsyncTasksDateCreatedColumn,
		constants.AsyncTasksAccountIdColumn,
		constants.AsyncTasksDateModifiedColumn,
		table,
	))
	if err != nil {
		t.Fatalf("Failed to query async tasks: %v", err)
	}
	defer rows.Close()

	ast = []models.AsyncTask{}
	for rows.Next() {
		var task models.AsyncTask
		err := rows.Scan(
			&task.RequestId,
			&task.Input,
			&task.Output,
			&task.State,
			&task.Service,
			&task.DateCreated,
			&task.AccountId,
			&task.DateModified,
		)
		if err != nil {
			t.Fatalf("Failed to scan async task: %v", err)
		}
		ast = append(ast, task)
	}

	assert.Equal(t, 0, len(ast))
}
