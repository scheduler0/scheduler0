package domains

import (
	"context"
	"cron-server/server/db"
	"fmt"
	"testing"
	"time"
)

var (
	projectOne = Project{}
	projectTwo = Project{Name: "Fake Project One", Description: "another fake project"}
	projectCtx = context.Background()
)

func TestProject_CreateOne(t *testing.T) {
	var projectsPool, _ = db.NewPool(db.CreateConnection, 1)
	defer projectsPool.Close()

	t.Log("Don't create project with name and description empty")
	{
		pid, err := projectOne.CreateOne(projectsPool, projectCtx)
		fmt.Println("pid", pid)
		if err == nil {
			t.Fatalf("\t\t Cannot create project without name and descriptions")
		}
	}

	t.Log("Create project with name and description not empty")
	{
		pid, err := projectTwo.CreateOne(projectsPool, projectCtx)
		fmt.Println("pid", pid)
		if err != nil {
			t.Fatalf("\t\t Error creating project %v", err)
		}
	}

	t.Log("Don't create project with existing name")
	{
		pid, err := projectTwo.CreateOne(projectsPool, projectCtx)
		fmt.Println("pid", pid)
		if err == nil {
			t.Fatalf("\t\t Cannot create project with existing name")
		}
	}
}

func TestProject_GetOne(t *testing.T) {
	var projectsPool, _ = db.NewPool(db.CreateConnection, 1)
	defer projectsPool.Close()

	t.Log("Can retrieve as single project")
	{
		projectTwoPlaceholder := Project{ID: projectTwo.ID}

		_, err := projectTwoPlaceholder.GetOne(projectsPool, projectCtx, "id = ?", projectTwoPlaceholder.ID)
		if err != nil {
			t.Fatalf("\t\t Could not get project %v", err)
		}

		if projectTwoPlaceholder.Name != projectTwo.Name {
			t.Fatalf("\t\t Projects name do not match Got = %v Expected = %v", projectTwoPlaceholder.Name, projectTwo.Name)
		}
	}
}

func TestProject_UpdateOne(t *testing.T) {
	var projectsPool, _ = db.NewPool(db.CreateConnection, 1)
	defer projectsPool.Close()

	t.Log("Can update name and description for a project")
	{
		projectTwo.Name = "some new name"

		_, err := projectTwo.UpdateOne(projectsPool, projectCtx)
		if err != nil {
			t.Fatalf("\t\t Could not update project %v", err)
		}

		projectTwoPlaceholder := Project{ID: projectTwo.ID}
		_, err = projectTwoPlaceholder.GetOne(projectsPool, projectCtx, "id = ?", projectTwoPlaceholder.ID)
		if err != nil {
			t.Fatalf("\t\t Could not get project with id %v", projectTwoPlaceholder.ID)
		}

		if projectTwoPlaceholder.Name != projectTwo.Name {
			t.Fatalf("\t\t Project names do not match  Expected %v to equal %v", projectTwo.Name, projectTwoPlaceholder.Name)
		}
	}
}

func TestProject_DeleteOne(t *testing.T) {
	var projectsPool, _ = db.NewPool(db.CreateConnection, 1)
	defer projectsPool.Close()

	t.Log("Delete All Projects")
	{
		rowsAffected, err := projectTwo.DeleteOne(projectsPool, projectCtx)
		if err != nil && rowsAffected > 0 {
			t.Fatalf("\t\t Cannot delete project one %v", err)
		}
	}

	t.Log("Don't delete project with a job")
	{
		projectOne.Name = "Untitled #1"
		projectOne.Description = "Pretty important project"
		id, err := projectOne.CreateOne(projectsPool, projectCtx)

		var job = Job{}
		job.ProjectId = id
		job.StartDate = time.Now().Add(60 * time.Second)
		job.CronSpec = "* * * * *"
		job.CallbackUrl = "https://some-random-url"

		_, err = job.CreateOne(projectsPool, projectCtx)
		if err != nil {
			t.Fatalf("Cannot create job %v", err)
		}

		rowsAffected, err := projectOne.DeleteOne(projectsPool, projectCtx)
		if err == nil || rowsAffected > 0 {
			t.Fatalf("\t\t Projects with jobs shouldn't be deleted %v %v", err, rowsAffected)
		}

		rowsAffected, err = job.DeleteOne(projectsPool, projectCtx)
		if err != nil || rowsAffected < 1 {
			t.Fatalf("\t\t Could not delete job  %v %v", err, rowsAffected)
		}

		rowsAffected, err = projectOne.DeleteOne(projectsPool, projectCtx)
		if err != nil || rowsAffected < 1 {
			t.Fatalf("\t\t Could not delete project  %v %v", err, rowsAffected)
		}
	}
}
