package managers

import (
	"context"
	"cron-server/server/src/managers"
	"cron-server/server/tests"
	"testing"
	"time"
)

var (
	projectOne = managers.ProjectManager{}
	projectTwo = managers.ProjectManager{Name: "Fake Project One", Description: "another fake project"}
	projectCtx = context.Background()
)

func TestProject_Manager(t *testing.T) {
	pool := tests.GetTestPool()

	t.Log("ProjectManager.CreateOne")
	{
		t.Logf("\t\tDon't create project with name and description empty")
		{
			_, err := projectOne.CreateOne(pool)
			if err == nil {
				t.Fatalf("\t\t Cannot create project without name and descriptions")
			}
		}

		t.Logf("\t\tCreate project with name and description not empty")
		{
			_, err := projectTwo.CreateOne(pool)
			if err != nil {
				t.Fatalf("\t\t Error creating project %v", err)
			}
		}

		t.Logf("\t\tDon't create project with existing name")
		{
			_, err := projectTwo.CreateOne(pool)
			if err == nil {
				t.Fatalf("\t\t Cannot create project with existing name")
			}
		}
	}

	t.Log("ProjectManager.UpdateOne")
	{
		t.Logf("\t\tCan retrieve as single project")
		{
			projectTwoPlaceholder := managers.ProjectManager{ID: projectTwo.ID}

			_, err := projectTwoPlaceholder.GetOne(pool, "id = ?", projectTwoPlaceholder.ID)
			if err != nil {
				t.Fatalf("\t\t Could not get project %v", err)
			}

			if projectTwoPlaceholder.Name != projectTwo.Name {
				t.Fatalf("\t\t Projects name do not match Got = %v Expected = %v", projectTwoPlaceholder.Name, projectTwo.Name)
			}
		}

		t.Logf("\t\tCan update name and description for a project")
		{
			projectTwo.Name = "some new name"

			_, err := projectTwo.UpdateOne(pool)
			if err != nil {
				t.Fatalf("\t\t Could not update project %v", err)
			}

			projectTwoPlaceholder := managers.ProjectManager{ID: projectTwo.ID}
			_, err = projectTwoPlaceholder.GetOne(pool, "id = ?", projectTwoPlaceholder.ID)
			if err != nil {
				t.Fatalf("\t\t Could not get project with id %v", projectTwoPlaceholder.ID)
			}

			if projectTwoPlaceholder.Name != projectTwo.Name {
				t.Fatalf("\t\t Project names do not match  Expected %v to equal %v", projectTwo.Name, projectTwoPlaceholder.Name)
			}
		}
	}

	t.Log("ProjectManager.DeleteOne")
	{
		t.Logf("\t\tDelete All Projects")
		{
			rowsAffected, err := projectTwo.DeleteOne(pool)
			if err != nil && rowsAffected > 0 {
				t.Fatalf("\t\t Cannot delete project one %v", err)
			}
		}

		t.Logf("\t\tDon't delete project with a job")
		{
			projectOne.Name = "Untitled #1"
			projectOne.Description = "Pretty important project"
			id, err := projectOne.CreateOne(pool)

			var job = managers.JobManager{}
			job.ProjectId = id
			job.StartDate = time.Now().Add(60 * time.Second)
			job.CronSpec = "* * * * *"
			job.CallbackUrl = "https://some-random-url"

			_, err = job.CreateOne(pool)
			if err != nil {
				t.Fatalf("Cannot create job %v", err)
			}

			rowsAffected, err := projectOne.DeleteOne(pool)
			if err == nil || rowsAffected > 0 {
				t.Fatalf("\t\t Projects with jobs shouldn't be deleted %v %v", err, rowsAffected)
			}

			rowsAffected, err = job.DeleteOne(pool)
			if err != nil || rowsAffected < 1 {
				t.Fatalf("\t\t Could not delete job  %v %v", err, rowsAffected)
			}

			rowsAffected, err = projectOne.DeleteOne(pool)
			if err != nil || rowsAffected < 1 {
				t.Fatalf("\t\t Could not delete project  %v %v", err, rowsAffected)
			}
		}
	}
}