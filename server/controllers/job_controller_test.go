package controllers

import (
	"context"
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/repository"
	"cron-server/server/testutils"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

var (
	jobController = JobController{}
	jobOne        models.Job
	jobTwo        models.Job
	project       models.Project
)

func TestJobController_CreateOne(t *testing.T) {
	testutils.TruncateDBBeforeTest()
	var jobsPool, err = repository.NewPool(repository.CreateConnection, 1)
	misc.CheckErr(err)
	jobController.Pool = *jobsPool

	t.Log("Respond with status 400 if request body does not contain required values")
	{
		jobOne.CronSpec = "* * * * *"
		jobByte, err := jobOne.ToJson()
		misc.CheckErr(err)
		jobStr := string(jobByte)
		if req, err := http.NewRequest("POST", "/jobs", strings.NewReader(jobStr)); err != nil {
			t.Fatalf("\t\t Cannot create http request")
		} else {
			w := httptest.NewRecorder()
			jobController.CreateOne(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		}
	}

	t.Log("Respond with status 201 if request body is valid")
	{
		project.Name = "TestJobController_Project"
		project.Description = "TestJobController_Project_Description"
		if id, err := project.CreateOne(&jobController.Pool, context.Background()); err != nil {
			t.Fatalf("\t\t Cannot create project %v", err)
		} else {
			project.ID = id
			j1 := models.InboundJob{}

			j1.CronSpec = "1 * * * *"
			j1.ProjectId = id
			j1.CallbackUrl = "http://random.url"
			j1.StartDate = time.Now().Add(60 * time.Second).UTC().Format(time.RFC1123)
			jobByte, err := j1.ToJson()
			misc.CheckErr(err)
			jobStr := string(jobByte)
			if req, err := http.NewRequest("POST", "/jobs", strings.NewReader(jobStr)); err != nil {
				t.Fatalf("\t\t Cannot create job %v", err)
			} else {
				w := httptest.NewRecorder()
				jobController.CreateOne(w, req)

				if body, err := ioutil.ReadAll(w.Body); err != nil {
					t.Fatalf("\t\t Could not read response body %v", err)
				} else {
					var response map[string]interface{}

					if err = json.Unmarshal(body, &response); err != nil {
						t.Fatalf("\t\t Could unmarsha json response %v", err)
					}

					log.Println(response)

					if len(response) < 1 {
						t.Fatalf("\t\t Response payload is empty")
					} else {
						jobOne.ID = response["data"].(string)
						assert.Equal(t, http.StatusCreated, w.Code)
					}
				}
			}
		}
	}
}

func TestJobController_GetAll(t *testing.T) {
	t.Log("Respond with status 200 and return all created jobs")
	{
		jobTwo.ProjectId = project.ID
		jobTwo.CronSpec = "1 * * * *"
		jobTwo.StartDate = time.Now().Add(60 * time.Second)

		rv := reflect.ValueOf(jobTwo)
		rt := rv.Type()
		rc := reflect.New(rt)
		rc.Elem().Set(rv)

		jobTwoCopy := rc.Interface().(*models.Job)

		if _, err := jobTwo.CreateOne(&jobController.Pool, context.Background()); err != nil {
			t.Fatalf("\t\t Cannot create job two")
		}

		if _, err := jobTwoCopy.CreateOne(&jobController.Pool, context.Background()); err != nil {
			t.Fatalf("\t\t Cannot create job three")
		}

		if req, err := http.NewRequest("GET", "/jobs?project_id="+project.ID, nil); err != nil {
			t.Fatalf("\t\t Cannot create http request")
		} else {
			w := httptest.NewRecorder()
			jobController.GetAll(w, req)
			assert.Equal(t, http.StatusOK, w.Code)

			if _, err := jobTwo.DeleteOne(&jobController.Pool, context.Background()); err != nil {
				t.Fatalf("\t\t Cannot delete job two")
			}

			if _, err := jobTwoCopy.DeleteOne(&jobController.Pool, context.Background()); err != nil {
				t.Fatalf("\t\t Cannot delete job two copy")
			}
		}
	}
}

func TestJobController_UpdateOne(t *testing.T) {
	t.Log("Respond with status 400 if update attempts to change cron spec")
	{
		jobOne.CronSpec = "3 * * * *"
		jobByte, err := jobOne.ToJson()
		misc.CheckErr(err)
		jobStr := string(jobByte)
		if req, err := http.NewRequest("PUT", "/jobs/"+jobOne.ID, strings.NewReader(jobStr)); err != nil {
			t.Fatalf("\t\t Cannot create http request")
		} else {
			w := httptest.NewRecorder()
			jobController.UpdateOne(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		}
	}

	t.Log("Respond with status 200 if update body is valid")
	{
		jobOne.CronSpec = "1 * * * *"
		jobOne.Description = "some job description"
		jobByte, err := jobOne.ToJson()
		misc.CheckErr(err)
		jobStr := string(jobByte)
		if req, err := http.NewRequest("PUT", "/jobs/"+jobOne.ID, strings.NewReader(jobStr)); err != nil {
			t.Fatalf("\t\t Cannot create http request")
		} else {
			w := httptest.NewRecorder()
			jobController.UpdateOne(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	}
}

func TestJobController_DeleteOne(t *testing.T) {
	t.Log("Respond with status 200 after successful deletion")
	{
		if req, err := http.NewRequest("DELETE", "/jobs/"+jobOne.ID, nil); err != nil {
			t.Fatalf("\t\t Cannot create http request")
		} else {
			w := httptest.NewRecorder()
			jobController.DeleteOne(w, req)
			assert.Equal(t, w.Code, http.StatusOK)

			if _, err = project.DeleteOne(&jobController.Pool, context.Background()); err != nil {
				t.Fatalf("\t\t Cannot delete project %v", err)
			}
		}
	}

	jobController.Pool.Close()
}
