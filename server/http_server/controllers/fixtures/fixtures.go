package fixtures

import (
	store2 "scheduler0/server/cluster"
	"scheduler0/server/db"
	"scheduler0/server/models"
	"scheduler0/server/repository"
	"scheduler0/server/repository/fixtures"
	"scheduler0/utils"
)

func CreateJobAndProjectManagerFixture() (models.ProjectModel, models.JobModel) {
	dbConnection := db.GetTestDBConnection()
	store := store2.NewStore(dbConnection, nil)
	jobRepo := repository.NewJobRepo(&store)
	projectRepo := repository.NewProjectRepo(&store, jobRepo)
	projectManager := fixtures.CreateProjectManagerFixture()
	_, createProjectError := projectRepo.CreateOne(projectManager)
	if createProjectError != nil {
		utils.Error(createProjectError.Message)
	}

	jobFixture := fixtures.JobFixture{}
	jobTransformers := jobFixture.CreateNJobTransformers(1)
	jobTransformer := jobTransformers[0]
	jobTransformer.ProjectID = projectManager.ID
	_, createJobError := jobRepo.CreateOne(jobTransformer)
	if createJobError != nil {
		utils.Error(createJobError.Message)
	}

	return projectManager, jobTransformer
}
