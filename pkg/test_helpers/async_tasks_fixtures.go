package test_helpers

import (
	"database/sql"
	"fmt"
	"github.com/brianvoe/gofakeit/v6"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	"testing"
)

func CreateFakeAsyncTasks(n int, conn *sql.DB, t *testing.T) []models.AsyncTask {
	var asyncTasksRes []models.AsyncTask
	for i := 0; i < n; i++ {
		var ast models.AsyncTask
		gofakeit.Struct(&ast)
		asyncTasksRes = append(asyncTasksRes, ast)
	}
	return asyncTasksRes
}

func InsertFakeAsyncTasks(n int, committed bool, conn *sql.DB, t *testing.T) []models.AsyncTask {
	table := constants.CommittedAsyncTableName
	if !committed {
		table = constants.UnCommittedAsyncTableName
	}

	var asyncTasksRes []models.AsyncTask
	for i := 0; i < n; i++ {
		var ast models.AsyncTask
		gofakeit.Struct(&ast)
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?)",
			table,
			constants.AsyncTasksRequestIdColumn,
			constants.AsyncTasksDateCreatedColumn,
			constants.AsyncTasksOutputColumn,
			constants.AsyncTasksStateColumn,
			constants.AsyncTasksServiceColumn,
			constants.AsyncTasksDateCreatedColumn,
		)
		params := []interface{}{
			ast.RequestId,
			ast.Input,
			ast.Output,
			ast.State,
			ast.Service,
			ast.DateCreated,
		}

		res, err := conn.Exec(query, params...)
		if err != nil {
			t.Fatalf("failed to create async tasks %v", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatalf("failed to create async tasks %v", err)
		}
		ast.Id = uint64(id)
		asyncTasksRes = append(asyncTasksRes, ast)
	}
	return asyncTasksRes
}

func GetAllAsyncTasks(committed bool, conn *sql.DB, t *testing.T) []models.AsyncTask {
	var results []models.AsyncTask
	table := constants.CommittedAsyncTableName
	if !committed {
		table = constants.UnCommittedAsyncTableName
	}

	query := fmt.Sprintf(
		"select %s, %s, %s, %s, %s, %s, %s from %s",
		constants.AsyncTasksIdColumn,
		constants.AsyncTasksRequestIdColumn,
		constants.AsyncTasksInputColumn,
		constants.AsyncTasksOutputColumn,
		constants.AsyncTasksStateColumn,
		constants.AsyncTasksServiceColumn,
		constants.AsyncTasksDateCreatedColumn,
		table,
	)
	rows, err := conn.Query(query)
	if err != nil {
		t.Fatalf("failed to run query %v", err)
	}
	for rows.Next() {
		var asyncTask models.AsyncTask
		scanErr := rows.Scan(
			&asyncTask.Id,
			&asyncTask.RequestId,
			&asyncTask.Input,
			&asyncTask.Output,
			&asyncTask.State,
			&asyncTask.Service,
			&asyncTask.DateCreated,
		)
		if scanErr != nil {
			t.Fatalf("failed to run query %v", scanErr)
		}
		results = append(results, asyncTask)
	}
	if rows.Err() != nil {
		t.Fatalf("failed to run query %v", rows.Err())
	}
	closeErr := rows.Close()
	if closeErr != nil {
		t.Fatalf("failed to run query %v", closeErr)
	}

	return results
}
