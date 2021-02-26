package fixtures

import (
	manager "scheduler0/server/managers/job"
	jobTestFixtures "scheduler0/server/managers/job/fixtures"
	"scheduler0/server/managers/project"
	projectTestFixtures "scheduler0/server/managers/project/fixtures"
	"scheduler0/utils"
)

func CreateJobAndProjectManagerFixture(pool *utils.Pool) (project.ProjectManager, manager.Manager) {
	projectManager := projectTestFixtures.CreateProjectManagerFixture()
	_, createProjectError := projectManager.CreateOne(pool)
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
	_, createJobError := jobManager.CreateOne(pool)
	if createJobError != nil {
		utils.Error(createJobError.Message)
	}

	return projectManager, jobManager
}
