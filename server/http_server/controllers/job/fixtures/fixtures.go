package fixtures

import (
	"github.com/go-pg/pg"
	manager "scheduler0/server/managers/job"
	jobTestFixtures "scheduler0/server/managers/job/fixtures"
	"scheduler0/server/managers/project"
	projectTestFixtures "scheduler0/server/managers/project/fixtures"
	"scheduler0/utils"
)

func CreateJobAndProjectManagerFixture(dbConnection *pg.DB) (project.ProjectManager, manager.Manager) {
	projectManager := projectTestFixtures.CreateProjectManagerFixture()
	_, createProjectError := projectManager.CreateOne(dbConnection)
	if createProjectError != nil {
		utils.Error(createProjectError.Message)
	}

	jobFixture := jobTestFixtures.JobFixture{}
	jobTransformers := jobFixture.CreateNJobTransformers(1)
	jobTransformer := jobTransformers[0]

	jobManager, transformJobManagerError := jobTransformer.ToManager()
	if transformJobManagerError != nil {
		utils.Error(transformJobManagerError)
	}
	jobManager.ProjectUUID = projectManager.UUID
	jobManager.ProjectID = projectManager.ID
	_, createJobError := jobManager.CreateOne(dbConnection)
	if createJobError != nil {
		utils.Error(createJobError.Message)
	}

	return projectManager, jobManager
}
