package test_helpers

import (
	"database/sql"
	"fmt"
	"github.com/brianvoe/gofakeit/v6"
	"scheduler0/constants"
	"scheduler0/models"
	"testing"
	"time"
)

func InsertFakeJobs(n int, projectIds []int64, conn *sql.DB, t *testing.T) []int64 {
	var ids []int64
	for i := 0; i < n; i++ {
		var j models.JobModel
		gofakeit.Struct(&j)
		j.DateCreated = time.Now()
		j.ProjectID = uint64(projectIds[i%len(projectIds)])
		params := []interface{}{
			j.ProjectID,
			j.Spec,
			j.DateCreated,
			j.CallbackUrl,
			j.Timezone,
			j.TimezoneOffset,
		}
		rs, err := conn.Exec(fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s) VALUES (?, ?, ?, ?, ?, ?)",
			constants.JobsTableName,
			constants.JobsProjectIdColumn,
			constants.JobsSpecColumn,
			constants.JobsDateCreatedColumn,
			constants.JobsCallbackURLColumn,
			constants.JobsTimezoneColumn,
			constants.JobsTimezoneOffsetColumn,
		), params...)
		if err != nil {
			t.Fatalf("Failed to insert fake job: %v", err)
		}
		jobId, err := rs.LastInsertId()
		if err != nil {
			t.Fatalf("Failed to insert fake job: %v", err)
		}
		ids = append(ids, jobId)
	}
	return ids
}
