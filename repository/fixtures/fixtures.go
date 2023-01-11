package fixtures

// CreateJobFixture creates a project and job for testing
//func CreateJobFixture(dbConnection *sql.DB) *models2.JobModel {
//	projectFixture := ProjectFixture{}
//	err := faker.FakeData(&projectFixture)
//	utils.CheckErr(err)
//
//	projectModel := models2.ProjectModel{
//		Name:        projectFixture.Name,
//		Description: projectFixture.Description,
//	}
//
//	store := store.Store{
//		SQLDbConnection: dbConnection,
//	}
//	jobRepo := repository2.NewJobRepo(&store)
//	projectRepo := repository2.NewProjectRepo(&store, jobRepo)
//
//	_, projectModelError := projectRepo.CreateOne(projectModel)
//	if projectModelError != nil {
//		utils.Error(projectModelError.Message)
//	}
//
//	jobFixture := JobFixture{}
//	jobTransformers := jobFixture.CreateNJobModels(1)
//	jobTransformers[0].ProjectID = projectModel.ID
//
//	_, createJobManagerError := jobRepo.CreateOne(jobTransformers[0])
//	if createJobManagerError != nil {
//		utils.Error(createJobManagerError.Message)
//		log.Fatalln(errors.New(createJobManagerError.Message))
//	}
//
//	return &jobTransformers[0]
//}
