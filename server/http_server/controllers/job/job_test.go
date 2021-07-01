package job_test

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robfig/cron"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"scheduler0/server/db"
	"scheduler0/server/http_server/controllers/job"
	jobTestFixtures "scheduler0/server/http_server/controllers/job/fixtures"
	jobManagerTestFixtures "scheduler0/server/managers/job/fixtures"
	projectTestFixtures "scheduler0/server/managers/project/fixtures"
	"scheduler0/server/process"
	"scheduler0/server/transformers"
	"scheduler0/utils"
	"strings"
	"testing"
)

var _ = Describe("Job Controller", func() {

	DBConnection := db.GetTestDBConnection()
	jobProcessor := process.JobProcessor{
		DBConnection: DBConnection,
		Cron: cron.New(),
		RecoveredJobs: []process.RecoveredJob{},
	}

	BeforeEach(func() {
		db.Teardown()
		db.Prepare()
	})

	Context("TestJobController_CreateOne", func() {

		It("Respond with status 400 if request body does not contain required values", func() {
			jobController := job.Controller{
				DBConnection: DBConnection,
				JobProcessor: &jobProcessor,
			}
			jobFixture := jobManagerTestFixtures.JobFixture{}
			jobTransformers := jobFixture.CreateNJobTransformers(1)
			jobByte, err := jobTransformers[0].ToJSON()
			utils.CheckErr(err)
			jobStr := string(jobByte)

			req, err := http.NewRequest("POST", "/jobs", strings.NewReader(jobStr))
			if err != nil {
				utils.Error("Cannot create http request")
			}

			w := httptest.NewRecorder()
			jobController.CreateOne(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("Respond with status 201 if request body is valid", func() {
			projectManager := projectTestFixtures.CreateProjectManagerFixture()
			projectManager.CreateOne(DBConnection)

			jobFixture := jobManagerTestFixtures.JobFixture{}
			jobTransformers := jobFixture.CreateNJobTransformers(1)
			jobTransformers[0].ProjectUUID = projectManager.UUID
			jobByte, err := jobTransformers[0].ToJSON()

			if err != nil {
				utils.Error(fmt.Sprintf("Cannot create job %v", err))
			}

			jobStr := string(jobByte)
			req, err := http.NewRequest("POST", "/jobs", strings.NewReader(jobStr))
			if err != nil {
				utils.Error(fmt.Sprintf("Cannot create job %v", err))
			}

			w := httptest.NewRecorder()

			controller := job.Controller{
				DBConnection: DBConnection,
				JobProcessor: &jobProcessor,
			}
			controller.CreateOne(w, req)
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

	Context("TestJobController_GetAll", func() {
		It("Respond with status 200 and return all created jobs", func() {
			projectTransformers := projectTestFixtures.CreateProjectTransformerFixture()
			projectManager := projectTransformers.ToManager()
			projectManager.CreateOne(DBConnection)
			n := 5

			jobFixture := jobManagerTestFixtures.JobFixture{}
			jobTransformers := jobFixture.CreateNJobTransformers(n)

			for i := 0; i < n; i++ {
				jobManager, err := jobTransformers[i].ToManager()
				if err != nil {
					utils.CheckErr(err)
				}
				jobManager.ProjectUUID = projectManager.UUID
				jobManager.CreateOne(DBConnection)
			}

			req, err := http.NewRequest("GET", "/jobs?offset=0&limit=10&projectUUID="+projectManager.UUID, nil)
			if err != nil {
				utils.Error(fmt.Sprintf("Cannot create http request %v", err))
			}

			w := httptest.NewRecorder()
			controller := job.Controller{DBConnection: DBConnection}
			controller.List(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Context("TestJobController_UpdateOne", func() {

		It("Respond with status 400 if update attempts to change cron spec", func() {
			_, jobManager := jobTestFixtures.CreateJobAndProjectManagerFixture(DBConnection)
			jobTransformer := transformers.Job{}
			jobTransformer.FromManager(jobManager)
			jobTransformer.Spec = "* * 3 * *"
			jobByte, err := jobTransformer.ToJSON()
			utils.CheckErr(err)
			jobStr := string(jobByte)
			req, err := http.NewRequest("PUT", "/jobs/"+jobTransformer.UUID, strings.NewReader(jobStr))
			if err != nil {
				utils.Error(fmt.Sprintf("Cannot create http request %v", err))
			}

			w := httptest.NewRecorder()
			controller := job.Controller{DBConnection: DBConnection}

			controller.UpdateOne(w, req)
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("Respond with status 200 if update body is valid", func() {
			_, jobManager := jobTestFixtures.CreateJobAndProjectManagerFixture(DBConnection)
			jobTransformer := transformers.Job{}
			jobTransformer.FromManager(jobManager)
			jobByte, err := jobTransformer.ToJSON()
			jobStr := string(jobByte)

			req, err := http.NewRequest("PUT", "/jobs/"+jobTransformer.UUID, strings.NewReader(jobStr))

			if err != nil {
				utils.Error(fmt.Sprintf("Cannot create http request %v", err))
			}

			w := httptest.NewRecorder()
			controller := job.Controller{DBConnection: DBConnection}
			router := mux.NewRouter()
			router.HandleFunc("/jobs/{uuid}", controller.UpdateOne)
			router.ServeHTTP(w, req)

			_, err = ioutil.ReadAll(w.Body)
			if err != nil {
				utils.Error(fmt.Sprintf("Cannot create http request %v", err))
			}

			Expect(w.Code).To(Equal(http.StatusOK))
		})

	})

	It("TestJobController_DeleteOne", func() {
		_, jobManager := jobTestFixtures.CreateJobAndProjectManagerFixture(DBConnection)

		req, err := http.NewRequest("DELETE", "/jobs/"+jobManager.UUID, nil)
		if err != nil {
			utils.Error(fmt.Sprintf("cannot create request to delete job %v", err))
		}

		w := httptest.NewRecorder()
		controller := job.Controller{
			DBConnection: DBConnection,
			JobProcessor: &jobProcessor,
		}

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{uuid}", controller.DeleteOne)
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
