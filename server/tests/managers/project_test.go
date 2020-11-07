package managers

import (
	"context"
	"cron-server/server/src/managers"
	"cron-server/server/tests"
	"github.com/magiconair/properties/assert"
	"strconv"
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
				t.Fatalf("\t\t [ERROR] Cannot create project without name and descriptions")
			}
		}

		t.Logf("\t\tCreate project with name and description not empty")
		{
			_, err := projectTwo.CreateOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Error creating project %v", err)
			}
		}

		t.Logf("\t\tDon't create project with existing name")
		{
			_, err := projectTwo.CreateOne(pool)
			if err == nil {
				t.Fatalf("\t\t [ERROR] Cannot create project with existing name")
			}
		}
	}

	t.Log("ProjectManager.UpdateOne")
	{
		t.Logf("\t\tCan retrieve as single project")
		{
			projectTwoPlaceholder := managers.ProjectManager{ID: projectTwo.ID}

			_, err := projectTwoPlaceholder.GetOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Could not get project %v", err)
			}

			if projectTwoPlaceholder.Name != projectTwo.Name {
				t.Fatalf("\t\t [ERROR] Projects name do not match Got = %v Expected = %v", projectTwoPlaceholder.Name, projectTwo.Name)
			}
		}

		t.Logf("\t\tCan update name and description for a project")
		{
			projectTwo.Name = "some new name"

			_, err := projectTwo.UpdateOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Could not update project %v", err)
			}

			projectTwoPlaceholder := managers.ProjectManager{ID: projectTwo.ID}
			_, err = projectTwoPlaceholder.GetOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Could not get project with id %v", projectTwoPlaceholder.ID)
			}

			if projectTwoPlaceholder.Name != projectTwo.Name {
				t.Fatalf("\t\t [ERROR] Project names do not match  Expected %v to equal %v", projectTwo.Name, projectTwoPlaceholder.Name)
			}
		}
	}

	t.Log("ProjectManager.DeleteOne")
	{
		t.Logf("\t\tDelete All Projects")
		{
			rowsAffected, err := projectTwo.DeleteOne(pool)
			if err != nil && rowsAffected > 0 {
				t.Fatalf("\t\t [ERROR] Cannot delete project one %v", err)
			}
		}

		t.Logf("\t\tDon't delete project with a job")
		{
			projectOne.Name = "Untitled #1"
			projectOne.Description = "Pretty important project"
			id, err := projectOne.CreateOne(pool)

			var job = managers.JobManager{}
			job.ProjectID = id
			job.StartDate = time.Now().Add(60 * time.Second)
			job.CronSpec = "* * * * *"
			job.CallbackUrl = "https://some-random-url"

			_, err = job.CreateOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Cannot create job %v", err)
			}

			rowsAffected, err := projectOne.DeleteOne(pool)
			if err == nil || rowsAffected > 0 {
				t.Fatalf("\t\t [ERROR] Projects with jobs shouldn't be deleted %v %v", err, rowsAffected)
			}

			rowsAffected, err = job.DeleteOne(pool, job.ID)
			if err != nil || rowsAffected < 1 {
				t.Fatalf("\t\t [ERROR] Could not delete job  %v %v", err, rowsAffected)
			}

			rowsAffected, err = projectOne.DeleteOne(pool)
			if err != nil || rowsAffected < 1 {
				t.Fatalf("\t\t [ERROR] Could not delete project  %v %v", err, rowsAffected)
			}
		}
	}

	t.Log("ProjectManager.GetAll")
	{
		manager := managers.ProjectManager{}

		for i := 0; i < 1000; i++ {
			project := managers.ProjectManager{
				Name: "project " + strconv.Itoa(i),
				Description: "project description "+ strconv.Itoa(i),
			}

			_, err := project.CreateOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] failed to create a project" + err.Error())
			}
		}

		projects, err := manager.GetAll(pool,  0, 100)
		if err != nil {
			t.Fatalf("\t\t [ERROR] failed to fetch projects" + err.Error())
		}

		assert.Equal(t, len(projects), 100)
	}
}