package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/victorlenerd/scheduler0/server/src/controllers"
	"github.com/victorlenerd/scheduler0/server/src/transformers"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"github.com/victorlenerd/scheduler0/server/tests"
	"github.com/victorlenerd/scheduler0/server/tests/fixtures"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJobController_CreateOne(t *testing.T) {
	pool := tests.GetTestPool()

	t.Log("Respond with status 400 if request body does not contain required values")
	{
		jobController := controllers.JobController{ Pool: pool }
		inboundJob := transformers.Job{}
		inboundJob.CronSpec = "* * * * *"
		jobByte, err := inboundJob.ToJson()
		utils.CheckErr(err)
		jobStr := string(jobByte)

		req, err := http.NewRequest("POST", "/jobs", strings.NewReader(jobStr))
		if err != nil {
			t.Fatalf("\t\t Cannot create http request")
		}

		w := httptest.NewRecorder()
		jobController.CreateJob(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	}

	t.Log("Respond with status 201 if request body is valid")
	{
		project := transformers.Project{}
		project.Name = "TestJobController_Project"
		project.Description = "TestJobController_Project_Description"

		projectManager := project.ToManager()

		projectID, err := projectManager.CreateOne(pool)

		if err != nil {
			t.Fatalf("\t\t Cannot create project %v", err)
		}

		j1 := transformers.Job{}

		j1.CronSpec = "1 * * * *"
		j1.ProjectID = projectID
		j1.CallbackUrl = "http://random.url"
		j1.StartDate = time.Now().Add(60 * time.Second).UTC().Format(time.RFC3339)
		jobByte, err := j1.ToJson()
		utils.CheckErr(err)
		jobStr := string(jobByte)
		req, err := http.NewRequest("POST", "/jobs", strings.NewReader(jobStr))
		if err != nil {
			t.Fatalf("\t\t Cannot create job %v", err)
		}

		w := httptest.NewRecorder()

		controller := controllers.JobController{ Pool: pool }
		controller.CreateJob(w, req)
		body, err := ioutil.ReadAll(w.Body)

		if err != nil {
			t.Fatalf("\t\t Could not read response body %v", err)
		}

		var response map[string]interface{}

		if err = json.Unmarshal(body, &response); err != nil {
			t.Fatalf("\t\t Could unmarsha json response %v", err)
		}

		if len(response) < 1 {
			t.Fatalf("\t\t Response payload is empty")
		}

		fmt.Println(response)
		assert.Equal(t, http.StatusCreated, w.Code)
	}
}

func TestJobController_GetAll(t *testing.T) {
	pool := tests.GetTestPool()


	t.Log("Respond with status 200 and return all created jobs")
	{

		project := transformers.Project{}
		project.Name = "TestJobController_GetAll"
		project.Description = "TestJobController_GetAll"

		projectManager:= project.ToManager()

		projectID, err := projectManager.CreateOne(pool)
		if err != nil {
			t.Fatalf("\t\t Cannot create project using manager %v", err)
		}

		jobModel := transformers.Job{}

		startDate := time.Now().Add(60 * time.Second).UTC().Format(time.RFC3339)

		jobModel.ProjectID = projectID
		jobModel.CronSpec = "1 * * * *"
		jobModel.StartDate = startDate
		jobModel.CallbackUrl = "some-url"

		jobManager, err := jobModel.ToManager()
		if err != nil {
			t.Fatalf("\t\t Failed to create job manager %v", err)
		}

		_, err = jobManager.CreateOne(pool)
		req, err := http.NewRequest("GET", "/jobs?offset=0&limit=10&projectID="+projectID, nil)

		if err != nil {
			t.Fatalf("\t\t Cannot create http request %v", err)
		}

		w := httptest.NewRecorder()
		controller := controllers.JobController{ Pool: pool }
		controller.ListJobs(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		body, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatalf("\t\t Error reading transformers %v", err)
		}

		fmt.Println(string(body))
	}
}

func TestJobController_UpdateOne(t *testing.T) {
	pool := tests.GetTestPool()

	t.Log("Respond with status 400 if update attempts to change cron spec")
	{
		inboundJob := transformers.Job{}

		inboundJob.CronSpec = "3 * * * *"
		jobByte, err := inboundJob.ToJson()
		utils.CheckErr(err)
		jobStr := string(jobByte)
		req, err := http.NewRequest("PUT", "/jobs/"+inboundJob.ID, strings.NewReader(jobStr))
		if err != nil {
			t.Fatalf("\t\t Cannot create http request %v", err)
		}

		w := httptest.NewRecorder()
		controller := controllers.JobController{ Pool: pool }

		controller.UpdateJob(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}

	t.Log("Respond with status 200 if update body is valid")
	{
		project := fixtures.CreateProjectFixture(pool, t)

		startDate := time.Now().Add(60 * time.Second).UTC().Format(time.RFC3339)

		inboundJob := transformers.Job{}
		inboundJob.StartDate = startDate
		inboundJob.CronSpec = "1 * * * *"
		inboundJob.Description = "some job description"
		inboundJob.Timezone = "UTC"
		inboundJob.ProjectID = project.ID
		inboundJob.CallbackUrl = "some-url"

		jobManager, err := inboundJob.ToManager()
		jobID, err := jobManager.CreateOne(pool)
		utils.CheckErr(err)

		updateJob := transformers.Job{}
		updateJob.ID = jobID
		updateJob.Description = "some new job description"
		updateJob.ProjectID = project.ID
		updateJob.CronSpec = "1 * * * *"
		updateJob.StartDate = time.Now().UTC().Format(time.RFC3339)
		jobByte, err := updateJob.ToJson()
		utils.CheckErr(err)

		jobStr := string(jobByte)

		req, err := http.NewRequest("PUT", "/jobs/"+jobID, strings.NewReader(jobStr))

		if err != nil {
			t.Fatalf("\t\t Cannot create http request %v", err)
		}

		w := httptest.NewRecorder()
		controller := controllers.JobController{ Pool: pool }
		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}", controller.UpdateJob)
		router.ServeHTTP(w, req)

		body, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Fatalf("\t\t Cannot create http request %v", err)
			log.Println("Response body :", string(body))
		}

		assert.Equal(t, http.StatusOK, w.Code)
		log.Println("Response body :", string(body))
	}
}

func TestJobController_DeleteOne(t *testing.T) {
	pool := tests.GetTestPool()


	t.Log("Respond with status 204 after successful deletion")
	{

		project := fixtures.CreateProjectFixture(pool, t)

		startDate := time.Now().Add(60 * time.Second).UTC().Format(time.RFC3339)

		inboundJob := transformers.Job{}
		inboundJob.StartDate = startDate
		inboundJob.CronSpec = "1 * * * *"
		inboundJob.Description = "some job description"
		inboundJob.Timezone = "UTC"
		inboundJob.ProjectID = project.ID
		inboundJob.CallbackUrl = "some-url"

		jobManager, err := inboundJob.ToManager()
		if err != nil {
			t.Fatalf("\t\t cannot create job manager from in-bound job %v", err)
		}
		jobID, err := jobManager.CreateOne(pool)
		if err != nil {
			t.Fatalf("\t\t cannot create job from job-manager %v", err)
		}

		req, err := http.NewRequest("DELETE", "/jobs/"+jobID, nil)
		if err != nil {
			t.Fatalf("\t\t cannot create request to delete job %v", err)
		}

		w := httptest.NewRecorder()
		controller := controllers.JobController{ Pool: pool }

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}", controller.DeleteJob)
		router.ServeHTTP(w, req)

		if err != nil {
			t.Fatalf("\t\t Cannot create http request %v", err)
		}

		assert.Equal(t, http.StatusNoContent, w.Code)
	}
}
