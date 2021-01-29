package fixtures

import (
	"github.com/bxcodec/faker/v3"
	"github.com/victorlenerd/scheduler0/server/src/managers"
	"github.com/victorlenerd/scheduler0/server/src/transformers"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"testing"
	"time"
)

type ProjectFixture struct {
	Name string `faker:"username"`
	Description string `faker:"username"`
}


func CreateProjectFixture(pool *utils.Pool, t *testing.T) transformers.Project {
	project := ProjectFixture{}
	err := faker.FakeData(&project)

	projectManager := managers.ProjectManager{
		Name: project.Name,
		Description: project.Description,
	}

	_, err = projectManager.CreateOne(pool)
	if err != nil {
		t.Fatalf("\t\t [ERROR] failed to create project:: %v", err.Error())
	}

	projectTransformer := transformers.Project{}
	projectTransformer.FromManager(projectManager)

	return projectTransformer
}

func CreateJobFixture(pool *utils.Pool, t *testing.T) transformers.Job {
	project := CreateProjectFixture(pool, t)

	jobManager := managers.JobManager{
		ProjectID: project.ID,
		StartDate: time.Now().Add(2000000),
		CallbackUrl: "https://some-random.url",
		CronSpec: "* * * * 1",
	}

	_, err := jobManager.CreateOne(pool)
	if err != nil {
		t.Fatalf("\t\t [ERROR] failed to create job %v", err.Error())
	}

	jobTransformer := transformers.Job{}
	jobTransformer.FromManager(jobManager)

	return jobTransformer
}