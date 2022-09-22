package controllers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robfig/cron"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"scheduler0/db"
	"scheduler0/fsm"
	jobController "scheduler0/http_server/controllers"
	jobTestFixtures "scheduler0/http_server/controllers/fixtures"
	"scheduler0/models"
	process2 "scheduler0/process"
	repository2 "scheduler0/repository"
	fixtures2 "scheduler0/repository/fixtures"
	"scheduler0/service"
	"scheduler0/utils"
	"strings"
	"testing"
)

var _ = Describe("Job Controller", func() {

	dbConnection := db.GetTestDBConnection()
	jobProcessor := process2.JobProcessor{
		DBConnection:  dbConnection,
		Cron:          cron.New(),
		RecoveredJobs: []process2.RecoveredJob{},
	}
	ctx := context.Background()

	store := fsm.NewFSMStore(nil, dbConnection)
	jobRepo := repository2.NewJobRepo(store)
	projectRepo := repository2.NewProjectRepo(store, jobRepo)
	jobService := service.NewJobService(jobRepo, ctx)
	controller := jobController.NewJoBHTTPController(jobService, jobProcessor)

	BeforeEach(func() {
		db.TeardownTestDB()
		db.PrepareTestDB()
	})

	Context("TestJobController_CreateOne", func() {

		It("Respond with status 400 if request body does not contain required values", func() {
			jobFixture := fixtures2.JobFixture{}
			jobTransformers := jobFixture.CreateNJobTransformers(1)
			jobByte, err := jobTransformers[0].ToJSON()
			utils.CheckErr(err)
			jobStr := string(jobByte)

			req, err := http.NewRequest("POST", "/jobs", strings.NewReader(jobStr))
			if err != nil {
				utils.Error("Cannot create http request")
			}

			w := httptest.NewRecorder()
			controller.CreateOneJob(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("Respond with status 201 if request body is valid", func() {
			projectManager := fixtures2.CreateProjectManagerFixture()
			projectRepo.CreateOne(projectManager)

			jobFixture := fixtures2.JobFixture{}
			jobTransformers := jobFixture.CreateNJobTransformers(1)
			jobTransformers[0].ProjectID = projectManager.ID
			jobByte, err := jobTransformers[0].ToJSON()

			if err != nil {
				utils.Error(fmt.Sprintf("Cannot create job %v", err))
			}

			jobStr := string(jobByte)
			req, err := http.NewRequest("POST", "/job", strings.NewReader(jobStr))
			if err != nil {
				utils.Error(fmt.Sprintf("Cannot create job %v", err))
			}

			w := httptest.NewRecorder()
			controller.CreateOneJob(w, req)
			body, err := ioutil.ReadAll(w.Body)

			if err != nil {
				utils.Error("Could not read response body %v", err)
			}

			var response map[string]interface{}

			if err = json.Unmarshal(body, &response); err != nil {
				utils.Error(fmt.Sprintf("Could unmarsha json response %v", err))
			}

			if len(response) < 1 {
				utils.Error("Response payload is empty")
			}

			utils.Info(response)

			Expect(w.Code).To(Equal(http.StatusCreated))
		})

	})

	Context("TestJobController_BatchCreate", func() {

	})

	Context("TestJobController_GetAll", func() {
		It("Respond with status 200 and return all created jobs", func() {
			projectTransformers := fixtures2.CreateProjectTransformerFixture()
			projectRepo.CreateOne(projectTransformers)
			n := 5

			jobFixture := fixtures2.JobFixture{}
			jobTransformers := jobFixture.CreateNJobTransformers(n)

			for i := 0; i < n; i++ {
				jobTransformers[i].ProjectID = projectTransformers.ID
				jobRepo.CreateOne(jobTransformers[i])
			}

			req, err := http.NewRequest("GET", fmt.Sprintf("/jobs?offset=0&limit=10&projectID=%v", projectTransformers.ID), nil)
			if err != nil {
				utils.Error(fmt.Sprintf("Cannot create http request %v", err))
			}

			w := httptest.NewRecorder()
			controller.ListJobs(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Context("TestJobController_UpdateOne", func() {

		It("Respond with status 400 if update attempts to change cron spec", func() {
			jobTransformer := models.JobModel{}
			jobTransformer.Spec = "* * 3 * *"
			jobByte, err := jobTransformer.ToJSON()
			utils.CheckErr(err)
			jobStr := string(jobByte)

			req, err := http.NewRequest("PUT", fmt.Sprintf("/jobs/%v", jobTransformer.ID), strings.NewReader(jobStr))
			if err != nil {
				utils.Error(fmt.Sprintf("Cannot create http request %v", err))
			}

			w := httptest.NewRecorder()

			controller.UpdateOneJob(w, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("Respond with status 200 if update body is valid", func() {
			_, jobManager := jobTestFixtures.CreateJobAndProjectManagerFixture()
			jobByte, err := jobManager.ToJSON()
			jobStr := string(jobByte)

			req, err := http.NewRequest("PUT", fmt.Sprintf("/jobs/%v", jobManager.ID), strings.NewReader(jobStr))

			if err != nil {
				utils.Error(fmt.Sprintf("Cannot create http request %v", err))
			}

			w := httptest.NewRecorder()
			router := mux.NewRouter()
			router.HandleFunc("/jobs/{id}", controller.UpdateOneJob)
			router.ServeHTTP(w, req)

			_, err = ioutil.ReadAll(w.Body)
			if err != nil {
				utils.Error(fmt.Sprintf("Cannot create http request %v", err))
			}

			Expect(w.Code).To(Equal(http.StatusOK))
		})

	})

	It("TestJobController_DeleteOne", func() {
		_, jobManager := jobTestFixtures.CreateJobAndProjectManagerFixture()

		req, err := http.NewRequest("DELETE", fmt.Sprintf("/jobs/%v", jobManager.ID), nil)
		if err != nil {
			utils.Error(fmt.Sprintf("cannot create request to delete job %v", err))
		}

		w := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}", controller.DeleteOneJob)
		router.ServeHTTP(w, req)

		if err != nil {
			utils.Error("Cannot create http request %v", err)
		}

		Expect(w.Code).To(Equal(http.StatusNoContent))
	})
})

func TestJob_Controller(t *testing.T) {
	utils.SetTestScheduler0Configurations()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Job Controller Suite")
}

func TestController_BatchCreateJobs(t *testing.T) {
	db.TeardownTestDB()
	db.PrepareTestDB()

	dbConnection := db.GetTestDBConnection()
	jobProcessor := process2.JobProcessor{
		DBConnection:  dbConnection,
		Cron:          cron.New(),
		RecoveredJobs: []process2.RecoveredJob{},
	}
	ctx := context.Background()

	store := fsm.NewFSMStore(nil, dbConnection)
	jobRepo := repository2.NewJobRepo(store)
	projectRepo := repository2.NewProjectRepo(store, jobRepo)
	jobService := service.NewJobService(jobRepo, ctx)
	//projectService := service.NewProjectService(projectRepo)
	controller := jobController.NewJoBHTTPController(jobService, jobProcessor)

	projectManager := fixtures2.CreateProjectManagerFixture()
	projectRepo.CreateOne(projectManager)

	jobFixture := fixtures2.JobFixture{}
	jobTransformers := jobFixture.CreateNJobTransformers(5)

	if data, err := json.Marshal(jobTransformers); err != nil {
		utils.Error("Failed to json marshal job transformers")
		Expect(err).To(BeNil())
	} else {
		jobStr := string(data)
		req, httpReqErr := http.NewRequest("POST", "/jobs", strings.NewReader(jobStr))
		if httpReqErr != nil {
			utils.Error(fmt.Sprintf("cannot create batch jobs %v", httpReqErr))
		}

		w := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/jobs", controller.BatchCreateJobs)
		router.ServeHTTP(w, req)

		if err != nil {
			utils.Error("cannot batch create http request %v", err)
		}

		body, reaErr := ioutil.ReadAll(w.Body)
		if reaErr != nil {
			utils.Error(fmt.Sprintf("Cannot create http request %v", reaErr))
		}

		utils.Info(string(body))

		if w.Code != http.StatusCreated {
			t.Fatalf("failed to create jobs: http status %v", w.Code)
		}
	}
}
