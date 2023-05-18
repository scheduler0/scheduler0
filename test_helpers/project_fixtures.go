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

func InsertFakeProjects(n int, conn *sql.DB, t *testing.T) []int64 {
	var ids []int64
	for i := 0; i < n; i++ {
		var p models.ProjectModel
		gofakeit.Struct(&p)
		p.DateCreated = time.Now()
		params := []interface{}{
			p.Name,
			p.Description,
			p.DateCreated,
		}
		rs, err := conn.Exec(fmt.Sprintf("INSERT INTO %s (%s, %s, %s) VALUES (?, ?, ?)",
			constants.ProjectsTableName,
			constants.ProjectsNameColumn,
			constants.ProjectsDescriptionColumn,
			constants.ProjectsDateCreatedColumn,
		), params...)
		if err != nil {
			t.Fatalf("Failed to insert project job: %v", err)
		}
		projectId, err := rs.LastInsertId()
		if err != nil {
			t.Fatalf("Failed to insert fake job: %v", err)
		}
		ids = append(ids, projectId)
	}
	return ids
}
