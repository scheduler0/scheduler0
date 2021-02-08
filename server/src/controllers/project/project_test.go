package project_test

import (
	"encoding/json"
	"fmt"
	"github.com/bxcodec/faker/v3"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/victorlenerd/scheduler0/server/src/controllers/project"
	jobTestFixtures "github.com/victorlenerd/scheduler0/server/src/controllers/job/fixtures"
	fixtures3 "github.com/victorlenerd/scheduler0/server/src/managers/project/fixtures"
	"github.com/victorlenerd/scheduler0/server/src/transformers"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"github.com/victorlenerd/scheduler0/server/tests"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)


var _ = Describe("Project Controller", func() {

	BeforeEach(func() {
		tests.Teardown()
		tests.Prepare()
	})

	pool := tests.GetTestPool()
	projectController := project.ProjectController{Pool: pool}

	It("Cannot create a project without name and description", func () {
		projectTransformer := transformers.Project{}
		projectOneJson, err := projectTransformer.ToJson()
		utils.CheckErr(err)
		projectOneJsonStr := strings.NewReader(string(projectOneJson))

		if req, err := http.NewRequest("POST", "/projects", projectOneJsonStr); err != nil {
			utils.Error("Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			projectController.CreateOne(w, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		}
	})

	It("Should create a new project with unique name and a description", func () {
		project := fixtures3.ProjectFixture{}
		err := faker.FakeData(&project)
		utils.CheckErr(err)

		projectTransformer := transformers.Project{
			Name:        project.Name,
			Description: project.Description,
		}

		projectJson, err := projectTransformer.ToJson()
		utils.CheckErr(err)
		projectJsonStr := strings.NewReader(string(projectJson))

		if req, err := http.NewRequest("POST", "/projects", projectJsonStr); err != nil {
			utils.Error("Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			projectController.CreateOne(w, req)
			if body, err := ioutil.ReadAll(w.Body); err != nil {
				utils.Error("\t\t Could not read response body %v", err)
			} else {
				var response map[string]interface{}

				if err = json.Unmarshal(body, &response); err != nil {
					utils.Error("\t\t Could not unmarsha json response %v", err)
				}

				if len(response) < 1 {
					utils.Error("\t\t Response payload is empty")
				} else {
					Expect(w.Code).To(Equal(http.StatusCreated))
				}
			}
		}
	})

	It("Cannot create project with the same name", func () {
		project := fixtures3.ProjectFixture{}
		err := faker.FakeData(&project)
		Expect(err).To(BeNil())

		projectTransformer := transformers.Project{
			Name:        project.Name,
			Description: project.Description,
		}

		projectManager := projectTransformer.ToManager()
		_, createOneProjectError := projectManager.CreateOne(pool)
		Expect(createOneProjectError).To(BeNil())

		projectJson, err := projectTransformer.ToJson()
		Expect(err).To(BeNil())
		projectJsonStr := strings.NewReader(string(projectJson))

		if req, err := http.NewRequest("POST", "/projects", projectJsonStr); err != nil {
			utils.Error("\t\t Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			projectController.CreateOne(w, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		}
	})

	It("Delete project without job", func () {
		project := fixtures3.ProjectFixture{}
		err := faker.FakeData(&project)
		if err != nil {
			utils.Error(err.Error())
		}

		projectTransformer := fixtures3.CreateProjectTransformerFixture()
		projectTransformer.Name = project.Name
		projectTransformer.Description = project.Description
		projectManager := projectTransformer.ToManager()
		projectManagerUUID, createProjectManagerError := projectManager.CreateOne(pool)
		if createProjectManagerError != nil {
			utils.Error(createProjectManagerError.Message)
		}

		if req, err := http.NewRequest("DELETE", "/projects/"+projectManagerUUID, nil); err != nil {
			utils.Error("\t\t Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			router := mux.NewRouter()
			router.HandleFunc("/projects/{uuid}", projectController.DeleteOne)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
		}
	})

	It("Should not maintain uniqueness of project names", func () {
		project := fixtures3.ProjectFixture{}
		err := faker.FakeData(&project)
		Expect(err).To(BeNil())
		if err != nil {
			utils.Error(err)
		}

		projectTransformer := transformers.Project{
			Name:        project.Name,
			Description: project.Description,
		}

		projectOneManager := projectTransformer.ToManager()
		_, createOneError := projectOneManager.CreateOne(pool)
		Expect(createOneError).To(BeNil())

		if createOneError != nil {
			utils.Error(createOneError.Message)
		}

		projectJson, err := projectTransformer.ToJson()
		Expect(err).To(BeNil())
		if err != nil {
			utils.Error(err.Error())
		}
		projectJsonStr := strings.NewReader(string(projectJson))
		utils.Error("projectOneJsonStr", projectJsonStr)

		if req, err := http.NewRequest("PUT", "/projects/"+projectOneManager.UUID, projectJsonStr); err != nil {
			utils.Error("\t\t request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			router := mux.NewRouter()
			router.HandleFunc("/projects/{uuid}", projectController.UpdateOne)
			router.ServeHTTP(w, req)

			_, err := ioutil.ReadAll(w.Body)
			if err != nil {
				utils.Error("\t\t %v", err.Error())
			}

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		}
	})

	It("Update name and description of an existing project", func () {
		project := fixtures3.ProjectFixture{}
		err := faker.FakeData(&project)
		Expect(err).To(BeNil())

		projectTransformer := transformers.Project{
			Name:        project.Name,
			Description: project.Description,
		}

		projectOneManager := projectTransformer.ToManager()
		projectOneManagerUUID, createProjectManagerError := projectOneManager.CreateOne(pool)
		if createProjectManagerError != nil {
			utils.Error(createProjectManagerError.Message)
		}
		Expect(createProjectManagerError).To(BeNil())

		err = faker.FakeData(&project)
		Expect(err).To(BeNil())
		utils.Error(err)

		projectTransformer = transformers.Project{
			Name:        project.Name,
			Description: project.Description,
		}

		projectJson, err := projectTransformer.ToJson()
		utils.Error(err)
		Expect(err).To(BeNil())
		projectJsonStr := strings.NewReader(string(projectJson))

		if req, err := http.NewRequest("PUT", "/projects/"+projectOneManagerUUID, projectJsonStr); err != nil {
			Expect(err).To(BeNil())
			utils.Error(fmt.Sprintf("Request failed %v", err))
		} else {
			w := httptest.NewRecorder()
			router := mux.NewRouter()
			router.HandleFunc("/projects/{uuid}", projectController.UpdateOne)
			router.ServeHTTP(w, req)

			_, err := ioutil.ReadAll(w.Body)
			if err != nil {
				utils.Error(err.Error())
				Expect(err).To(BeNil())
			}

			Expect(w.Code).To(Equal(http.StatusOK))
		}
	})


	It("Get all projects with the same name or description", func () {
		if req, err := http.NewRequest("GET", "/projects?limit=10&offset=0", nil); err != nil {
			utils.Error("\t\t Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			projectController.GetAll(w, req)
			if _, err := ioutil.ReadAll(w.Body); err != nil {
				utils.Error(err.Error())
				Expect(err).To(BeNil())
			} else {
				Expect(w.Code).To(Equal(http.StatusOK))
			}
		}
	})


	It("Do not delete projects with jobs ", func () {
		_, jobManager := jobTestFixtures.CreateJobAndProjectManagerFixture(pool)

		if req, err := http.NewRequest("DELETE", "/projects/"+jobManager.ProjectUUID, nil); err != nil {
			utils.Error("\t\t Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			router := mux.NewRouter()
			router.HandleFunc("/projects/{uuid}", projectController.DeleteOne)
			router.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		}
	})
})

func TestProject_Controller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Project Controller Suite")
}