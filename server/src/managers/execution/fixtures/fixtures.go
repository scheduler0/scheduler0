package fixtures

import (
	"errors"
	"github.com/bxcodec/faker/v3"
	"scheduler0/server/src/managers/job"
	fixtures2 "scheduler0/server/src/managers/job/fixtures"
	"scheduler0/server/src/managers/project"
	fixtures3 "scheduler0/server/src/managers/project/fixtures"
	"scheduler0/server/src/utils"
)

// CreateJobFixture creates a project and job for testing
func CreateJobFixture(pool *utils.Pool) *job.JobManager {
	projectFixture := fixtures3.ProjectFixture{}
	err := faker.FakeData(&projectFixture)
	utils.CheckErr(err)

	projectManager := project.ProjectManager{
		Name:        projectFixture.Name,
		Description: projectFixture.Description,
	}
	_, projectManagerError := projectManager.CreateOne(pool)
	if projectManagerError != nil {
		utils.Error(projectManagerError.Message)
	}

	jobFixture := fixtures2.JobFixture{}
	jobTransformers := jobFixture.CreateNJobTransformers(1)
	jobManager, toManagerError := jobTransformers[0].ToManager()
	if toManagerError != nil {
		utils.Error(toManagerError)
	}

	jobManager.ProjectUUID = projectManager.UUID
	jobManager.ID = projectManager.ID

	_, createJobManagerError := jobManager.CreateOne(pool)
	if createJobManagerError != nil {
		utils.Error(createJobManagerError.Message)
		panic(errors.New(createJobManagerError.Message))
	}

	return &jobManager
}
