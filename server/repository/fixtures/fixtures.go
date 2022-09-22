package fixtures

import (
	"database/sql"
	"errors"
	"github.com/bxcodec/faker/v3"
	"log"
	store "scheduler0/server/cluster"
	"scheduler0/server/models"
	"scheduler0/server/repository"
	"scheduler0/utils"
)

// CreateJobFixture creates a project and job for testing
func CreateJobFixture(dbConnection *sql.DB) *models.JobModel {
	projectFixture := ProjectFixture{}
	err := faker.FakeData(&projectFixture)
	utils.CheckErr(err)

	projectModel := models.ProjectModel{
		Name:        projectFixture.Name,
		Description: projectFixture.Description,
	}

	store := store.Store{
		SqliteDB: dbConnection,
	}
	jobRepo := repository.NewJobRepo(&store)
	projectRepo := repository.NewProjectRepo(&store, jobRepo)

	_, projectModelError := projectRepo.CreateOne(projectModel)
	if projectModelError != nil {
		utils.Error(projectModelError.Message)
	}

	jobFixture := JobFixture{}
	jobTransformers := jobFixture.CreateNJobTransformers(1)
	jobTransformers[0].ProjectID = projectModel.ID

	_, createJobManagerError := jobRepo.CreateOne(jobTransformers[0])
	if createJobManagerError != nil {
		utils.Error(createJobManagerError.Message)
		log.Fatalln(errors.New(createJobManagerError.Message))
	}

	return &jobTransformers[0]
}
