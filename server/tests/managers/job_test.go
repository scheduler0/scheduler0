package managers

import (
	"context"
	"cron-server/server/src/managers"
	"cron-server/server/tests"
	"testing"
	"time"
)

var (
	project  = managers.ProjectManager{Name: "test project", Description: "test project"}
	jobOne   = managers.JobManager{}
	jobTwo   = managers.JobManager{}
	jobThree = managers.JobManager{}
	jobCtx   = context.Background()
)

func TestJob_CreateOne(t *testing.T) {
	pool := tests.GetTestPool()

	t.Log("Creating job returns error if required inbound fields are nil")
	{
		jobOne.CallbackUrl = "http://test-url"
		jobOne.Data = "some-transformers"
		jobOne.ProjectId = ""
		jobOne.CronSpec = "* * * * *"

		_, err := jobOne.CreateOne(pool)

		if err == nil {
			t.Fatalf("\t\t  Model should require values")
		}
	}

	t.Log("Creating job returns error if project id does not exist")
	{
		jobTwo.CallbackUrl = "http://test-url"
		jobTwo.Data = "some-transformers"
		jobTwo.ProjectId = "test-project-id"
		jobTwo.StartDate = time.Now().Add(600000 * time.Second)
		jobTwo.CronSpec = "* * * * *"

		id, err := jobTwo.CreateOne(pool)

		if err == nil {
			t.Fatalf("\t\t  Invalid project id does not exist but job with %v was created", id)
		}
	}

	t.Log("Creating job returns new id")
	{
		id, err := project.CreateOne(pool)
		if err != nil {
			t.Fatalf("\t\t  Cannot create project %v", err)
		}

		if len(id) < 1 {
			t.Fatalf("\t\t  Project id is invalid %v", id)
		}

		project.ID = id
		jobThree.CallbackUrl = "http://test-url"
		jobThree.Data = "some-transformers"
		jobThree.ProjectId = id
		jobThree.StartDate = time.Now().Add(600000 * time.Second)
		jobThree.CronSpec = "* * * * *"

		_, err = jobThree.CreateOne(pool)
		if err != nil {
			t.Fatalf("\t\t  Could not create job %v", err)
		}

		rowsAffected, err := jobThree.DeleteOne(pool)
		if err != nil {
			t.Fatalf("\t\t Could not delete job %v", err)
		}

		rowsAffected, err = project.DeleteOne(pool)
		if err != nil && rowsAffected < 1 {
			t.Fatalf("\t\t  Could not delete project %v", err)
		}
	}
}

func TestJob_UpdateOne(t *testing.T) {
	pool := tests.GetTestPool()

	t.Log("Cannot update cron spec on job")
	{
		id, err := project.CreateOne(pool)
		if err != nil {
			t.Fatalf("\t\t  Cannot create project %v", err)
		}

		if len(id) < 1 {
			t.Fatalf("\t\t  Project id is invalid %v", id)
		}

		jobThree.ProjectId = id
		jobThree.CronSpec = "1 * * * *"

		id, err = jobThree.CreateOne(pool)
		if err != nil {
			t.Fatalf("\t\t Could not update job %v", err)
		}

		jobThree.CronSpec = "2 * * * *"
		_, err = jobThree.UpdateOne(pool)
		if err == nil {
			t.Fatalf("\t\t Could not update job %v", err)
		}

		jobThreePlaceholder := managers.JobManager{ID: jobThree.ID}
		_, err = jobThreePlaceholder.GetOne(pool, "id = ?", jobThree.ID)
		if err != nil {
			t.Fatalf("\t\t Could not get job %v", err)
		}

		if jobThreePlaceholder.CronSpec == jobThree.CronSpec {
			t.Fatalf("\t\t CronSpec should be immutable")
		}

		_, err = jobThree.DeleteOne(pool)
		if err != nil {
			t.Fatalf("\t\t Could not update job %v", err)
		}

		_, err = project.DeleteOne(pool)
		if err != nil {
			t.Fatalf("\t\t Could not update job %v", err)
		}
	}
}

func TestJob_DeleteOne(t *testing.T) {
	pool := tests.GetTestPool()

	t.Log("Delete jobs")
	{
		rowsAffected, err := jobThree.DeleteOne(pool)
		if err != nil && rowsAffected > 0 {
			t.Fatalf("\t\t %v", err)
		}

		rowsAffected, err = project.DeleteOne(pool)
		if err != nil && rowsAffected > 0 {
			t.Fatalf("\t\t %v", err)
		}
	}
}
