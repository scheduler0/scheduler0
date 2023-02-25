package controllers_test

//var _ = Describe("Project Controller", func() {
//
//	dbConnection := db.GetTestDBConnection()
//	store := fsm.NewFSMStore(nil, dbConnection)
//	jobRepo := repository2.NewJobRepo(store)
//	projectRepo := repository2.NewProjectRepo(store, jobRepo)
//	projectService := service.NewProjectService(projectRepo)
//	controller := controllers.NewProjectController(projectService)
//
//	BeforeEach(func() {
//		db.TeardownTestDB()
//		db.PrepareTestDB()
//	})
//
//	It("Cannot create a project without name and description", func() {
//		projectTransformer := models.ProjectModel{}
//		projectOneJSON, err := projectTransformer.ToJSON()
//		utils.CheckErr(err)
//		projectOneJSONStr := strings.NewReader(string(projectOneJSON))
//
//		if req, err := http.NewRequest("POST", "/projects", projectOneJSONStr); err != nil {
//			utils.Error("Request failed %v", err)
//		} else {
//			w := httptest.NewRecorder()
//			controller.CreateOneProject(w, req)
//			Expect(w.Code).To(Equal(http.StatusBadRequest))
//		}
//	})
//
//	It("Should create a new project with unique name and a description", func() {
//		project := fixtures3.ProjectFixture{}
//		err := faker.FakeData(&project)
//		utils.CheckErr(err)
//
//		projectTransformer := models.ProjectModel{
//			Name:        project.Name,
//			Description: project.Description,
//		}
//
//		projectJSON, err := projectTransformer.ToJSON()
//		utils.CheckErr(err)
//		projectJSONStr := strings.NewReader(string(projectJSON))
//
//		if req, err := http.NewRequest("POST", "/projects", projectJSONStr); err != nil {
//			utils.Error("Request failed %v", err)
//		} else {
//			w := httptest.NewRecorder()
//			controller.CreateOneProject(w, req)
//			if body, err := ioutil.ReadAll(w.Body); err != nil {
//				utils.Error("\t\t Could not read response body %v", err)
//			} else {
//				var response map[string]interface{}
//
//				if err = json.Unmarshal(body, &response); err != nil {
//					utils.Error("\t\t Could not unmarsha json response %v", err)
//				}
//
//				if len(response) < 1 {
//					utils.Error("\t\t Response payload is empty")
//				} else {
//					Expect(w.Code).To(Equal(http.StatusCreated))
//				}
//			}
//		}
//	})
//
//	It("Cannot create project with the same name", func() {
//		project := fixtures3.ProjectFixture{}
//		err := faker.FakeData(&project)
//		Expect(err).To(BeNil())
//
//		projectTransformer := models.ProjectModel{
//			Name:        project.Name,
//			Description: project.Description,
//		}
//		_, createOneProjectError := projectRepo.CreateOne(projectTransformer)
//		Expect(createOneProjectError).To(BeNil())
//
//		projectJSON, err := projectTransformer.ToJSON()
//		Expect(err).To(BeNil())
//		projectJSONStr := strings.NewReader(string(projectJSON))
//
//		if req, err := http.NewRequest("POST", "/projects", projectJSONStr); err != nil {
//			utils.Error("\t\t Request failed %v", err)
//		} else {
//			w := httptest.NewRecorder()
//			controller.CreateOneProject(w, req)
//			Expect(w.Code).To(Equal(http.StatusBadRequest))
//		}
//	})
//
//	It("Delete project without job", func() {
//		project := fixtures3.ProjectFixture{}
//		err := faker.FakeData(&project)
//		if err != nil {
//			utils.Error(err.Error())
//		}
//
//		projectTransformer := fixtures3.CreateProjectTransformerFixture()
//		projectTransformer.Name = project.Name
//		projectTransformer.Description = project.Description
//		projectManagerUUID, createProjectManagerError := projectRepo.CreateOne(projectTransformer)
//		if createProjectManagerError != nil {
//			utils.Error(createProjectManagerError.Message)
//		}
//
//		if req, err := http.NewRequest("DELETE", fmt.Sprintf("/projects/%v", projectManagerUUID), nil); err != nil {
//			utils.Error("\t\t Request failed %v", err)
//		} else {
//			w := httptest.NewRecorder()
//			router := mux.NewRouter()
//			router.HandleFunc("/projects/{id}", controller.DeleteOneProject)
//			router.ServeHTTP(w, req)
//
//			Expect(w.Code).To(Equal(http.StatusNoContent))
//		}
//	})
//
//	It("Should not maintain uniqueness of project names", func() {
//		project := fixtures3.ProjectFixture{}
//		err := faker.FakeData(&project)
//		Expect(err).To(BeNil())
//		if err != nil {
//			utils.Error(err)
//		}
//
//		projectTransformer := models.ProjectModel{
//			Name:        project.Name,
//			Description: project.Description,
//		}
//		_, createOneError := projectRepo.CreateOne(projectTransformer)
//		Expect(createOneError).To(BeNil())
//
//		if createOneError != nil {
//			utils.Error(createOneError.Message)
//		}
//
//		projectJSON, err := projectTransformer.ToJSON()
//		Expect(err).To(BeNil())
//		if err != nil {
//			utils.Error(err.Error())
//		}
//		projectJSONStr := strings.NewReader(string(projectJSON))
//		utils.Error("projectOneJSONStr", projectJSONStr)
//
//		if req, err := http.NewRequest("PUT", fmt.Sprintf("/projects/%v", projectTransformer.ID), projectJSONStr); err != nil {
//			utils.Error("\t\t request failed %v", err)
//		} else {
//			w := httptest.NewRecorder()
//			router := mux.NewRouter()
//			router.HandleFunc("/projects/{uuid}", controller.UpdateOneProject)
//			router.ServeHTTP(w, req)
//
//			_, err := ioutil.ReadAll(w.Body)
//			if err != nil {
//				utils.Error("\t\t %v", err.Error())
//			}
//
//			Expect(w.Code).To(Equal(http.StatusBadRequest))
//		}
//	})
//
//	It("Update name and description of an existing project", func() {
//		project := fixtures3.ProjectFixture{}
//		err := faker.FakeData(&project)
//		Expect(err).To(BeNil())
//
//		projectTransformer := models.ProjectModel{
//			Name:        project.Name,
//			Description: project.Description,
//		}
//
//		projectOneManagerUUID, createProjectManagerError := projectRepo.CreateOne(projectTransformer)
//		if createProjectManagerError != nil {
//			utils.Error(createProjectManagerError.Message)
//		}
//		Expect(createProjectManagerError).To(BeNil())
//
//		err = faker.FakeData(&project)
//		Expect(err).To(BeNil())
//		utils.Error(err)
//
//		projectTransformer = models.ProjectModel{
//			Name:        project.Name,
//			Description: project.Description,
//		}
//
//		projectJSON, err := projectTransformer.ToJSON()
//		utils.Error(err)
//		Expect(err).To(BeNil())
//		projectJSONStr := strings.NewReader(string(projectJSON))
//
//		if req, err := http.NewRequest("PUT", fmt.Sprintf("/projects/%v", projectOneManagerUUID), projectJSONStr); err != nil {
//			Expect(err).To(BeNil())
//			utils.Error(fmt.Sprintf("Request failed %v", err))
//		} else {
//			w := httptest.NewRecorder()
//			router := mux.NewRouter()
//			router.HandleFunc("/projects/{id}", controller.UpdateOneProject)
//			router.ServeHTTP(w, req)
//
//			_, err := ioutil.ReadAll(w.Body)
//			if err != nil {
//				utils.Error(err.Error())
//				Expect(err).To(BeNil())
//			}
//
//			Expect(w.Code).To(Equal(http.StatusOK))
//		}
//	})
//
//	It("Get all projects with the same name or description", func() {
//		if req, err := http.NewRequest("GET", "/projects?limit=10&offset=0", nil); err != nil {
//			utils.Error("\t\t Request failed %v", err)
//		} else {
//			w := httptest.NewRecorder()
//			controller.ListProjects(w, req)
//			if _, err := ioutil.ReadAll(w.Body); err != nil {
//				utils.Error(err.Error())
//				Expect(err).To(BeNil())
//			} else {
//				Expect(w.Code).To(Equal(http.StatusOK))
//			}
//		}
//	})
//
//	It("Do not delete projects with jobs ", func() {
//		_, jobManager := jobTestFixtures.CreateJobAndProjectManagerFixture()
//
//		if req, err := http.NewRequest("DELETE", fmt.Sprintf("/projects/%v", jobManager.ProjectID), nil); err != nil {
//			utils.Error("\t\t Request failed %v", err)
//		} else {
//			w := httptest.NewRecorder()
//			router := mux.NewRouter()
//			router.HandleFunc("/projects/{id}", controller.DeleteOneProject)
//			router.ServeHTTP(w, req)
//			Expect(w.Code).To(Equal(http.StatusBadRequest))
//		}
//	})
//})
//
//func TestProject_Controller(t *testing.T) {
//	utils.SetTestScheduler0Configurations()
//	RegisterFailHandler(Fail)
//	RunSpecs(t, "Project Controller Suite")
//}
