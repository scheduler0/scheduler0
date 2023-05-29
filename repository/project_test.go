package repository_test

//var _ = Describe("ProjectRepo Repo", func() {
//	pool := db.GetTestDBConnection()
//	fsmStore := store2.Store{
//		dataStore: pool,
//	}
//	jobRepo := repository2.NewJobRepo(&fsmStore)
//	projectRepo := repository2.NewProjectRepo(&fsmStore, jobRepo)
//
//	BeforeEach(func() {
//		db.TeardownTestDB()
//		db.PrepareTestDB()
//	})
//
//	It("Don't create project with name and description empty", func() {
//		projectModel := models2.ProjectModel{}
//		_, err := projectRepo.CreateOne(projectModel)
//		if err == nil {
//			utils.Error("[ERROR] Cannot create project without name and descriptions")
//		}
//		Expect(err).ToNot(BeNil())
//	})
//
//	It("Create project with name and description not empty", func() {
//		projectFixture := fixtures2.ProjectFixture{}
//		err := faker.FakeData(&projectFixture)
//		utils.CheckErr(err)
//
//		projectModel := models2.ProjectModel{
//			Name:        projectFixture.Name,
//			Description: projectFixture.Description,
//		}
//
//		_, createOneProjectError := projectRepo.CreateOne(projectModel)
//		if createOneProjectError != nil {
//			utils.Error(fmt.Sprintf("[ERROR] Error creating project %v", err))
//		}
//	})
//
//	It("Don't create project with existing name", func() {
//		projectFixture := fixtures2.ProjectFixture{}
//		err := faker.FakeData(&projectFixture)
//		utils.CheckErr(err)
//
//		projectManagerOne := models2.ProjectModel{
//			Name:        projectFixture.Name,
//			Description: projectFixture.Description,
//		}
//
//		projectManagerTwo := models2.ProjectModel{
//			Name:        projectFixture.Name,
//			Description: projectFixture.Description,
//		}
//
//		_, createOneError := projectRepo.CreateOne(projectManagerOne)
//		if createOneError != nil {
//			utils.Error(createOneError.Message)
//		}
//		Expect(createOneError).To(BeNil())
//
//		_, createTwoError := projectRepo.CreateOne(projectManagerTwo)
//		if createTwoError == nil {
//			utils.Error(fmt.Sprintf("[ERROR] Cannot create project with existing name"))
//		}
//		Expect(createTwoError).ToNot(BeNil())
//	})
//
//	It("Can retrieve a single project by uuid", func() {
//		projectFixture := fixtures2.ProjectFixture{}
//		err := faker.FakeData(&projectFixture)
//		utils.CheckErr(err)
//
//		projectModel := models2.ProjectModel{
//			Name:        projectFixture.Name,
//			Description: projectFixture.Description,
//		}
//		_, projectManagerError := projectRepo.CreateOne(projectModel)
//		if projectManagerError != nil {
//			utils.Error(projectManagerError.Message)
//		}
//		Expect(projectManagerError).To(BeNil())
//
//		projectManagerByName := models2.ProjectModel{
//			ID: projectModel.ID,
//		}
//
//		projectManagerByNameError := projectRepo.GetOneByID(&projectManagerByName)
//		if projectManagerByNameError != nil {
//			utils.Error(fmt.Sprintf("[ERROR] Could not get project %v", err))
//		}
//
//		Expect(projectManagerByName.ID).To(Equal(projectModel.ID))
//	})
//
//	It("Can retrieve a single project by name", func() {
//		projectFixture := fixtures2.ProjectFixture{}
//		err := faker.FakeData(&projectFixture)
//		utils.CheckErr(err)
//
//		projectModel := models2.ProjectModel{
//			Name:        projectFixture.Name,
//			Description: projectFixture.Description,
//		}
//		_, projectManagerError := projectRepo.CreateOne(projectModel)
//		if projectManagerError != nil {
//			utils.Error(projectManagerError.Message)
//		}
//		Expect(projectManagerError).To(BeNil())
//
//		projectByName := models2.ProjectModel{
//			Name: projectModel.Name,
//		}
//
//		projectManagerByNameError := projectRepo.GetOneByName(&projectByName)
//		if projectManagerByNameError != nil {
//			utils.Error(fmt.Sprintf("[ERROR] Could not get project %v", err))
//		}
//
//		Expect(projectByName.ID).To(Equal(projectModel.ID))
//		Expect(projectByName.Name).To(Equal(projectModel.Name))
//	})
//
//	It("Can update name and description for a project", func() {
//		projectFixture := fixtures2.ProjectFixture{}
//		err := faker.FakeData(&projectFixture)
//		utils.CheckErr(err)
//
//		projectModel := models2.ProjectModel{
//			Name:        projectFixture.Name,
//			Description: projectFixture.Description,
//		}
//		_, createProjectManagerError := projectRepo.CreateOne(projectModel)
//		if createProjectManagerError != nil {
//			utils.Error(createProjectManagerError.Message)
//		}
//		Expect(createProjectManagerError).To(BeNil())
//
//		projectFixture = fixtures2.ProjectFixture{}
//		err = faker.FakeData(&projectFixture)
//		utils.CheckErr(err)
//
//		projectTwoPlaceholder := models2.ProjectModel{
//			ID:   projectModel.ID,
//			Name: projectFixture.Name,
//		}
//		_, updateProjectError := projectRepo.UpdateOneByID(projectTwoPlaceholder)
//		if updateProjectError != nil {
//			utils.Error(updateProjectError.Message)
//		}
//
//		Expect(projectModel.Name).NotTo(Equal(projectTwoPlaceholder.Name))
//	})
//
//	It("Delete All Projects", func() {
//		projectFixture := fixtures2.ProjectFixture{}
//		err := faker.FakeData(&projectFixture)
//		utils.CheckErr(err)
//
//		projectManager := models2.ProjectModel{
//			Name:        projectFixture.Name,
//			Description: projectFixture.Description,
//		}
//		_, createProjectManagerError := projectRepo.CreateOne(projectManager)
//		if createProjectManagerError != nil {
//			utils.Error(createProjectManagerError.Message)
//		}
//
//		rowsAffected, deleteProjectOneError := projectRepo.DeleteOneByID(projectManager)
//		if deleteProjectOneError != nil || rowsAffected < 1 {
//			utils.Error(fmt.Sprintf("[ERROR] Cannot delete project one %v", err))
//		}
//	})
//
//	It("Don't delete project with a job", func() {
//		projectFixture := fixtures2.ProjectFixture{}
//		err := faker.FakeData(&projectFixture)
//		utils.CheckErr(err)
//
//		projectManager := models2.ProjectModel{
//			Name:        projectFixture.Name,
//			Description: projectFixture.Description,
//		}
//		_, createProjectManagerError := projectRepo.CreateOne(projectManager)
//		if createProjectManagerError != nil {
//			utils.Error(createProjectManagerError.Message)
//		}
//
//		job := models2.JobModel{
//			ProjectID:   projectManager.ID,
//			Spec:        "* * * * *",
//			CallbackUrl: "https://some-random-url",
//		}
//
//		_, createOneJobError := jobRepo.CreateOne(job)
//		if err != nil {
//			utils.Error(fmt.Sprintf("[ERROR] Cannot create job %v", createOneJobError.Message))
//		}
//
//		rowsAffected, deleteOneProjectError := projectRepo.DeleteOneByID(projectManager)
//		if deleteOneProjectError == nil || rowsAffected > 0 {
//			utils.Error(fmt.Sprintf("[ERROR] Projects with jobs shouldn't be deleted %v", rowsAffected))
//		}
//
//		rowsAffected, deleteOneJobError := jobRepo.DeleteOneByID(job)
//		if err != nil || rowsAffected < 1 {
//			utils.Error(fmt.Sprintf("[ERROR] Could not delete job  %v %v", deleteOneJobError.Message, rowsAffected))
//		}
//
//		rowsAffected, deleteOneProjectError = projectRepo.DeleteOneByID(projectManager)
//		if err != nil || rowsAffected < 1 {
//			utils.Error("[ERROR] Could not delete project %v", rowsAffected)
//		}
//	})
//
//	It("ProjectManager.ListByJobID", func() {
//		for i := 0; i < 1000; i++ {
//			project := models2.ProjectModel{
//				Name:        "project " + strconv.Itoa(i),
//				Description: "project description " + strconv.Itoa(i),
//			}
//
//			_, err := projectRepo.CreateOne(project)
//			if err != nil {
//				utils.Error("[ERROR] failed to create a project ::", err.Message)
//			}
//		}
//
//		projects, err := projectRepo.List(0, 100)
//		if err != nil {
//			utils.Error("[ERROR] failed to fetch projects ::", err.Message)
//		}
//
//		Expect(len(projects)).To(Equal(100))
//	})
//})
//
//func TestProject_Manager(t *testing.T) {
//	utils.SetTestScheduler0Configurations()
//	RegisterFailHandler(Fail)
//	RunSpecs(t, "ProjectRepo Repo Suite")
//}
