package fixtures

import (
	"cron-server/server/src/managers"
	"cron-server/server/src/utils"
	"testing"
	"time"
)

func CreateJobFixture(pool *utils.Pool, t *testing.T, ProjectName string) string {
	projectManager := managers.ProjectManager{
		Name: ProjectName,
		Description: "some random desc",
	}
	ProjectID, err := projectManager.CreateOne(pool)

	if err != nil {
		t.Fatalf("\t\t [ERROR] failed to create project:: %v", err.Error())
	}

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