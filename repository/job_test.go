package repository_test

//var _ = Describe("Job Repo", func() {
//	pool := db.GetTestDBConnection()
//	fsmStore := store2.Store{
//		DataStore: pool,
//	}
//	jobRepo := repository2.NewJobRepo(&fsmStore)
//	projectRepo := repository2.NewProjectRepo(&fsmStore, jobRepo)
//
//	BeforeEach(func() {
//		db.TeardownTestDB()
//		db.PrepareTestDB()
//	})
//
//	Context("JobRepo.CreateOne", func() {
//		It("Creating job returns error if required inbound fields are nil", func() {
//			jobFixture := fixtures2.JobFixture{}
//			jobTransformers := jobFixture.CreateNJobModels(1)
//
//			uuid, err := jobRepo.CreateOne(jobTransformers[0])
//			if err == nil {
//				utils.Error("[ERROR] Model should require values")
//			}
//
//			Expect(uuid).To(Equal(int64(-1)))
//		})
//
//		It("Creating job returns new id", func() {
//			jobFixture := fixtures2.JobFixture{}
//			jobTransformers := jobFixture.CreateNJobModels(1)
//			projectTransformer := fixtures2.CreateProjectTransformerFixture()
//			_, createOneProjectError := projectRepo.CreateOne(projectTransformer)
//			if createOneProjectError != nil {
//				utils.Error(fmt.Sprintf("[ERROR] Cannot create project %v", createOneProjectError.Message))
//			}
//
//			jobTransformers[0].ProjectID = projectTransformer.ID
//
//			id, err := jobRepo.CreateOne(jobTransformers[0])
//			if err != nil {
//				utils.Error(fmt.Sprintf("[ERROR] Cannot create job %v", err.Message))
//			}
//
//			if id != 0 && id != -1 {
//				utils.Error(fmt.Sprintf("[ERROR] Project uuid is invalid %v", id))
//			}
//		})
//	})
//
//	Context("JobRepo.UpdateOneByID", func() {
//		It("Cannot update cron spec on job", func() {
//			jobFixture := fixtures2.JobFixture{}
//			jobTransformers := jobFixture.CreateNJobModels(1)
//			projectTransformer := fixtures2.CreateProjectTransformerFixture()
//			_, createOneProjectError := projectRepo.CreateOne(projectTransformer)
//			if createOneProjectError != nil {
//				utils.Error(fmt.Sprintf("[ERROR] Cannot create project %v", createOneProjectError.Message))
//			}
//
//			jobTransformers[0].ProjectID = projectTransformer.ID
//
//			id, err := jobRepo.CreateOne(jobTransformers[0])
//			if err != nil {
//				utils.Error(fmt.Sprintf("[ERROR] Cannot create job %v", err.Message))
//			}
//
//			jobGetManager := models.JobModel{
//				ID: id,
//			}
//
//			jobGetManager.ProjectID = projectTransformer.ID
//			jobGetManager.Spec = "1 * * * *"
//
//			_, updateOneError := jobRepo.UpdateOneByID(jobGetManager)
//			if updateOneError == nil {
//				utils.Error("[ERROR] Job cron spec should not be replaced")
//			}
//		})
//	})
//
//	Context("JobRepo.DeleteOneByID", func() {
//		It("Delete jobs", func() {
//			jobFixture := fixtures2.JobFixture{}
//			jobTransformers := jobFixture.CreateNJobModels(1)
//			projectTransformer := fixtures2.CreateProjectTransformerFixture()
//			_, createOneProjectError := projectRepo.CreateOne(projectTransformer)
//			if createOneProjectError != nil {
//				utils.Error(fmt.Sprintf("[ERROR] Cannot create project %v", createOneProjectError.Message))
//			}
//
//			jobTransformers[0].ProjectID = projectTransformer.ID
//			jobRepo.CreateOne(jobTransformers[0])
//
//			rowsAffected, err := jobRepo.DeleteOneByID(jobTransformers[0])
//			if err != nil && rowsAffected > 0 {
//				utils.Error(err.Message)
//			}
//		})
//	})
//
//	It("JobRepo.ListByJobID", func() {
//		jobFixture := fixtures2.JobFixture{}
//		jobTransformers := jobFixture.CreateNJobModels(5)
//
//		projectTransformer := fixtures2.CreateProjectTransformerFixture()
//		_, createOneProjectError := projectRepo.CreateOne(projectTransformer)
//		if createOneProjectError != nil {
//			utils.Error(fmt.Sprintf("[ERROR] Cannot create project %v", createOneProjectError.Message))
//		}
//
//		for i := 0; i < 5; i++ {
//			jobTransformers[i].ProjectID = projectTransformer.ID
//			_, createOneJobRepoError := jobRepo.CreateOne(jobTransformers[i])
//			if createOneJobRepoError != nil {
//				utils.Error(fmt.Sprintf("[ERROR] To job manager %v", createOneJobRepoError.Message))
//			}
//		}
//
//		jobs, _, getAllJobsError := jobRepo.GetJobsPaginated(projectTransformer.ID, 0, 5)
//		if getAllJobsError != nil {
//			utils.Error(fmt.Sprintf("[ERROR] Cannot get all projects %v", getAllJobsError.Message))
//		}
//
//		Expect(len(jobs)).To(Equal(5))
//	})
//
//	It("JobRepo.GetOneID", func() {
//		jobFixture := fixtures2.JobFixture{}
//		jobTransformers := jobFixture.CreateNJobModels(1)
//		projectTransformer := fixtures2.CreateProjectTransformerFixture()
//		_, createOneProjectError := projectRepo.CreateOne(projectTransformer)
//		if createOneProjectError != nil {
//			utils.Error(fmt.Sprintf("[ERROR] Cannot create project %v", createOneProjectError.Message))
//		}
//		jobTransformers[0].ProjectID = projectTransformer.ID
//		jobRepo.CreateOne(jobTransformers[0])
//		getOneJobError := jobRepo.GetOneByID(&jobTransformers[0])
//		if getOneJobError != nil {
//			utils.Error(fmt.Sprintf("[ERROR]  Failed to get job by id %v", getOneJobError.Message))
//		}
//
//		Expect(jobTransformers[0].ProjectID).To(Equal(projectTransformer.ID))
//	})
//
//	It("JobRepo.BatchGet", func() {
//		jobFixture := fixtures2.JobFixture{}
//		jobTransformers := jobFixture.CreateNJobModels(5)
//
//		projectTransformer := fixtures2.CreateProjectTransformerFixture()
//
//		jobModels := []models.JobModel{}
//
//		for i := 0; i < 5; i++ {
//			jobTransformers[i].ProjectID = projectTransformer.ID
//			jobModels = append(jobModels, jobTransformers[i])
//		}
//
//		uuids, batchInsertErr := jobRepo.BatchInsertJobs(jobModels)
//		if batchInsertErr != nil {
//			utils.Error(fmt.Sprintf("[ERROR] failed to insert %v", batchInsertErr.Message))
//		}
//
//		fetchedJobRepos, batchGetErr := jobRepo.BatchGetJobsByID(uuids)
//		if batchGetErr != nil {
//			utils.Error(fmt.Sprintf("[ERROR] Failed to get %v", batchGetErr.Message))
//		}
//
//		Expect(len(fetchedJobRepos)).To(Equal(5))
//	})
//
//	It("Batch Insertion", func() {
//		jobFixture := fixtures2.JobFixture{}
//		jobTransformers := jobFixture.CreateNJobModels(5)
//
//		projectTransformer := fixtures2.CreateProjectTransformerFixture()
//
//		jobModels := []models.JobModel{}
//
//		for i := 0; i < 5; i++ {
//			jobTransformers[i].ProjectID = projectTransformer.ID
//			jobModels = append(jobModels, jobTransformers[i])
//		}
//
//		uuids, batchInsertErr := jobRepo.BatchInsertJobs(jobModels)
//		if batchInsertErr != nil {
//			utils.Error(fmt.Sprintf("[ERROR] To job manager %v", batchInsertErr.Message))
//		}
//
//		Expect(len(uuids)).To(Equal(5))
//	})
//})
//
//func TestJob_Manager(t *testing.T) {
//	utils.SetTestScheduler0Configurations()
//	RegisterFailHandler(Fail)
//	RunSpecs(t, "Job Repo Suite")
//}
