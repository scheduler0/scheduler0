package test_helpers

import (
	"database/sql"
	"fmt"
	"github.com/brianvoe/gofakeit/v6"
	"scheduler0/constants"
	"scheduler0/models"
	"testing"
)

func InsertFakeJobExecutionLogs(n int, jobIds []int64, conn *sql.DB, t *testing.T) []models.JobExecutionLog {
	var jobExecutionLogs []models.JobExecutionLog
	for i := 0; i < n; i++ {
		var uce models.JobExecutionLog
		gofakeit.Struct(&uce)
		uce.JobId = uint64(jobIds[i%len(jobIds)])
		params := []interface{}{
			uce.UniqueId,
			uce.State,
			uce.NodeId,
			uce.LastExecutionDatetime,
			uce.NextExecutionDatetime,
			uce.JobId,
			uce.DataCreated,
			uce.JobQueueVersion,
			uce.ExecutionVersion,
		}
		_, err := conn.Exec(fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);",
			constants.ExecutionsUnCommittedTableName,
			constants.ExecutionsUniqueIdColumn,
			constants.ExecutionsStateColumn,
			constants.ExecutionsNodeIdColumn,
			constants.ExecutionsLastExecutionTimeColumn,
			constants.ExecutionsNextExecutionTime,
			constants.ExecutionsJobIdColumn,
			constants.ExecutionsDateCreatedColumn,
			constants.ExecutionsJobQueueVersion,
			constants.ExecutionsVersion,
		), params...)
		if err != nil {
			t.Fatalf("Failed to insert fake uncommitted execution log: %v", err)
		}
		jobExecutionLogs = append(jobExecutionLogs, uce)
	}
	return jobExecutionLogs
}
