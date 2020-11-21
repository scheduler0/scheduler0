package fixtures

import (
	"cron-server/server/src/managers"
	"cron-server/server/src/utils"
	"github.com/bxcodec/faker/v3"
	"testing"
	"time"
)

type ProjectFixture struct {
	Name string `faker:"username"`
	Description string `faker:"username"`
}


func CreateProjectFixture(pool *utils.Pool, t *testing.T) string {
	project := ProjectFixture{}
	err := faker.FakeData(&project)

	projectManager := managers.ProjectManager{
		Name: project.Name,
		Description: project.Description,
	}

	ProjectID, err := projectManager.CreateOne(pool)
	if err != nil {
		t.Fatalf("\t\t [ERROR] failed to create project:: %v", err.Error())
	}

	return ProjectID
}

func CreateJobFixture(pool *utils.Pool, t *testing.T) string {
	ProjectID := CreateProjectFixture(pool, t)


	jobManager := managers.JobManager{
		ProjectID: ProjectID,
		StartDate: time.Now().Add(2000000),
		CallbackUrl: "https://some-random.url",
		CronSpec: "* * * * 1",
	}

	JobID, err := jobManager.CreateOne(pool)
	if err != nil {
		t.Fatalf("\t\t [ERROR] failed to create job %v", err.Error())
	}

	return JobID
}