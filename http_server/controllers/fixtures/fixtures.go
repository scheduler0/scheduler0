package fixtures

import (
	"scheduler0/db"
	store2 "scheduler0/fsm"
	models2 "scheduler0/models"
	repository2 "scheduler0/repository"
	fixtures2 "scheduler0/repository/fixtures"
	"scheduler0/utils"
)

func CreateJobAndProjectManagerFixture() (models2.ProjectModel, models2.JobModel) {
	dbConnection := db.GetTestDBConnection()
	store := store2.Store{
		SQLDbConnection: dbConnection,
	}
	jobRepo := repository2.NewJobRepo(&store)
	projectRepo := repository2.NewProjectRepo(&store, jobRepo)
	projectManager := fixtures2.CreateProjectManagerFixture()
	_, createProjectError := projectRepo.CreateOne(projectManager)
	if createProjectError != nil {
		utils.Error(createProjectError.Message)
	}

	jobFixture := fixtures2.JobFixture{}
	jobTransformers := jobFixture.CreateNJobTransformers(1)
	jobTransformer := jobTransformers[0]
	jobTransformer.ProjectID = projectManager.ID
	_, createJobError := jobRepo.CreateOne(jobTransformer)
	if createJobError != nil {
		utils.Error(createJobError.Message)
	}

	return projectManager, jobTransformer
}
