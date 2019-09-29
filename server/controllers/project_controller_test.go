package controllers

import (
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/testutils"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var projectController = ProjectController{}

var projectOne = models.Project{}
var projectTwo = models.Project{}
var projectOneJobOne = models.Job{}

func TestProjectController_CreateOne(t *testing.T) {
	testutils.TruncateDBBeforeTest()

	t.Log("Cannot create project without name and description")
	{
		projectOneJson := projectOne.ToJson()
		projectOneJsonStr := strings.NewReader(string(projectOneJson))

		if req, err := http.NewRequest("POST", "/projects", projectOneJsonStr); err != nil {
			t.Fatalf("Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			projectController.CreateOne(w, req)
			assert.Equal(t, w.Code, http.StatusBadRequest)
		}
	}

	t.Log("Create a new project with unique name and a description")
	{
		projectOne.Name = "Untitled Project #1"
		projectOne.Description = "a simple job funnel"

		projectOneJson := projectOne.ToJson()
		projectOneJsonStr := strings.NewReader(string(projectOneJson))

		if req, err := http.NewRequest("POST", "/projects", projectOneJsonStr); err != nil {
			t.Fatalf("Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			projectController.CreateOne(w, req)
			if body, err := ioutil.ReadAll(w.Body); err != nil {
				t.Fatalf("\t\t Could not read response body %v", err)
			} else {
				var response map[string]interface{}

				if err = json.Unmarshal(body, &response); err != nil {
					t.Fatalf("\t\t Could not unmarsha json response %v", err)
				}

				if len(response) < 1 {
					t.Fatalf("\t\t Response payload is empty")
				} else {
					projectOne.ID = response["data"].(string)
					assert.Equal(t, w.Code, http.StatusCreated)
				}
			}
		}
	}

	t.Log("Cannot create project with the same name")
	{
		projectOneJson := projectOne.ToJson()
		projectOneJsonStr := strings.NewReader(string(projectOneJson))

		if req, err := http.NewRequest("POST", "/projects", projectOneJsonStr); err != nil {
			t.Fatalf("\t\t Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			projectController.CreateOne(w, req)
			assert.Equal(t, w.Code, http.StatusBadRequest)
		}
	}
}

func TestProjectController_UpdateOne(t *testing.T) {
	t.Log("Cannot update name of project to name of an existing project")
	{
		projectTwo.Name = "Untitled Project #2"
		projectTwo.Description = "untitled project two description"

		if id, err := projectTwo.CreateOne(); err != nil {
			t.Fatalf("failed to create project two")
		} else {
			projectTwo.ID = id
			projectTwo.Name = "Untitled Project #1"
			projectTwoJson := projectTwo.ToJson()
			projectTwoJsonStr := strings.NewReader(string(projectTwoJson))

			if req, err := http.NewRequest("PUT", "/projects/"+projectTwo.ID, projectTwoJsonStr); err != nil {
				t.Fatalf("\t\t Request failed %v", err)
			} else {
				w := httptest.NewRecorder()
				projectController.UpdateOne(w, req)
				assert.Equal(t, w.Code, http.StatusBadRequest)
			}
		}
	}

	t.Log("Update name and description of an existing project")
	{
		projectTwo.Name = "Project #2"
		projectTwoJson := projectTwo.ToJson()
		projectTwoJsonStr := strings.NewReader(string(projectTwoJson))

		if req, err := http.NewRequest("PUT", "/projects/"+projectTwo.ID, projectTwoJsonStr); err != nil {
			t.Fatalf("\t\t Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			projectController.UpdateOne(w, req)
			assert.Equal(t, w.Code, http.StatusOK)
		}
	}
}

func TestProjectController_GetAll(t *testing.T) {
	t.Log("Get all projects with the same name or description")
	{
		if req, err := http.NewRequest("GET", "/projects?name=Untitled Project", nil); err != nil {
			t.Fatalf("\t\t Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			projectController.GetAll(w, req)
			if _, err := ioutil.ReadAll(w.Body); err != nil {
				misc.CheckErr(err)
			} else {
				assert.Equal(t, w.Code, http.StatusOK)
			}
		}
	}
}

func TestProjectController_DeleteOne(t *testing.T) {
	t.Log("Do not delete projects with jobs ")
	{
		projectOneJobOne.ProjectId = projectOne.ID
		projectOneJobOne.Description = "sample job"
		projectOneJobOne.CronSpec = "* * * * *"
		projectOneJobOne.StartDate = time.Now().Add(90 * time.Second)
		projectOneJobOne.CallbackUrl = "https://time.com"

		if _, err := projectOneJobOne.CreateOne(); err != nil {
			t.Fatalf("\t\t Could not create job")
		} else {
			if req, err := http.NewRequest("DELETE", "/projects/"+projectOne.ID, nil); err != nil {
				t.Fatalf("\t\t Request failed %v", err)
			} else {
				w := httptest.NewRecorder()
				projectController.DeleteOne(w, req)
				assert.Equal(t, w.Code, http.StatusBadRequest)
			}
		}
	}
}
