package managers

import (
	"cron-server/server/src/managers"
	"cron-server/server/tests"
	"github.com/magiconair/properties/assert"
	"testing"
	"time"
)

var (
	project  = managers.ProjectManager{Name: "test project", Description: "test project"}
	jobOne   = managers.JobManager{}
	jobTwo   = managers.JobManager{}
	jobThree = managers.JobManager{}
)

func TestJob_Manager(t *testing.T)  {
	pool := tests.GetTestPool()

	t.Log("JobManager.CreateOne")
	{
		t.Logf("\t\tCreating job returns error if required inbound fields are nil")
		{
			jobOne.CallbackUrl = "http://test-url"
			jobOne.Data = "some-transformers"
			jobOne.ProjectID = ""
			jobOne.CronSpec = "* * * * *"

			_, err := jobOne.CreateOne(pool)

			if err == nil {
				t.Fatalf("\t\t [ERROR] Model should require values")
			}
		}

		t.Logf("\t\tCreating job returns error if project id does not exist")
		{
			jobTwo.CallbackUrl = "http://test-url"
			jobTwo.Data = "some-transformers"
			jobTwo.ProjectID = "test-project-id"
			jobTwo.StartDate = time.Now().Add(600000 * time.Second)
			jobTwo.CronSpec = "* * * * *"

			id, err := jobTwo.CreateOne(pool)

			if err == nil {
				t.Fatalf("\t\t [ERROR] Invalid project id does not exist but job with %v was created", id)
			}
		}

		t.Logf("\t\tCreating job returns new id")
		{
			id, err := project.CreateOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Cannot create project %v", err)
			}

			if len(id) < 1 {
				t.Fatalf("\t\t [ERROR] Project id is invalid %v", id)
			}

			project.ID = id
			jobThree.CallbackUrl = "http://test-url"
			jobThree.Data = "some-transformers"
			jobThree.ProjectID = id
			jobThree.StartDate = time.Now().Add(600000 * time.Second)
			jobThree.CronSpec = "* * * * *"

			_, err = jobThree.CreateOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Could not create job %v", err)
			}

			rowsAffected, err := jobThree.DeleteOne(pool, jobThree.ID)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Could not delete job %v", err)
			}

			rowsAffected, err = project.DeleteOne(pool)
			if err != nil && rowsAffected < 1 {
				t.Fatalf("\t\t [ERROR] Could not delete project %v", err)
			}
		}
	}

	t.Log("JobManager.UpdateOne")
	{
		t.Logf("\t\tCannot update cron spec on job")
		{
			id, err := project.CreateOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Cannot create project %v", err)
			}

			if len(id) < 1 {
				t.Fatalf("\t\t [ERROR] Project id is invalid %v", id)
			}

			jobThree.ProjectID = id
			jobThree.CronSpec = "1 * * * *"

			id, err = jobThree.CreateOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Could not update job %v", err)
			}

			jobThree.CronSpec = "2 * * * *"
			_, err = jobThree.UpdateOne(pool)
			if err == nil {
				t.Fatalf("\t\t [ERROR] Could not update job %v", err)
			}

			jobThreePlaceholder := managers.JobManager{ID: jobThree.ID}
			err = jobThreePlaceholder.GetOne(pool, jobThree.ID)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Could not get job %v", err)
			}

			if jobThreePlaceholder.CronSpec == jobThree.CronSpec {
				t.Fatalf("\t\t [ERROR] CronSpec should be immutable")
			}

			_, err = jobThree.DeleteOne(pool, jobThree.ID)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Could not update job %v", err)
			}

			_, err = project.DeleteOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Could not update job %v", err)
			}
		}
	}

	t.Log("JobManager.DeleteOne")
	{
		t.Logf("\t\tDelete jobs")
		{
			rowsAffected, err := jobThree.DeleteOne(pool, jobThree.ID)
			if err != nil && rowsAffected > 0 {
				t.Fatalf("\t\t %v", err)
			}

			rowsAffected, err = project.DeleteOne(pool)
			if err != nil && rowsAffected > 0 {
				t.Fatalf("\t\t [ERROR]  %v", err)
			}
		}
	}

	t.Log("JobManager.GetAll")
	{
		_  , err := project.CreateOne(pool)

		jobThree.CallbackUrl = "http://test-url"
		jobThree.Data = "some-transformers"
		jobThree.ProjectID = project.ID
		jobThree.StartDate = time.Now().Add(600000 * time.Second)
		jobThree.CronSpec = "* * * * *"

		for i := 0; i < 1000; i++ {
			_, err := jobThree.CreateOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR]  Failed to create a job %v", err.Error())
			}
		}

		jobs, err := jobThree.GetAll(pool, project.ID, 0, 100, "date_created")
		if err != nil {
			t.Fatalf("\t\t [ERROR]  Failed to get all jobs %v", err.Error())
		}

		assert.Equal(t, len(jobs), 100)
	}

	t.Log("JobManager.GetOne")
	{

		jobThree.CallbackUrl = "http://test-url"
		jobThree.Data = "some-transformers"
		jobThree.ProjectID = project.ID
		jobThree.StartDate = time.Now().Add(600000 * time.Second)
		jobThree.CronSpec = "* * * * *"
		_, err := jobThree.CreateOne(pool)
		if err != nil {
			t.Fatalf("\t\t [ERROR]  Failed to create job %v", err.Error())
		}

		jobResult := managers.JobManager{}
		err = jobResult.GetOne(pool, jobThree.ID)
		if err != nil {
			t.Fatalf("\t\t [ERROR]  Failed to get job by id %v", err.Error())
		}

		assert.Equal(t, jobThree.CallbackUrl, jobResult.CallbackUrl)
	}
}
