package controllers

import (
	"context"
	"cron-server/server/src/controllers"
	"cron-server/server/src/transformers"
	"cron-server/server/src/utils"
	"cron-server/server/tests"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/magiconair/properties/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var (
	projectOne       = transformers.Project{}
	projectTwo       = transformers.Project{}
	projectOneJobOne = transformers.Job{}
	ctx              = context.Background()
)

func TestProjectController_CreateOne(t *testing.T) {
	pool := tests.GetTestPool()
	projectController := controllers.ProjectController{ Pool: pool }

	t.Log("Cannot create project without name and description")
	{
		projectOneJson, err := projectOne.ToJson()
		utils.CheckErr(err)
		projectOneJsonStr := strings.NewReader(string(projectOneJson))

		if req, err := http.NewRequest("POST", "/projects", projectOneJsonStr); err != nil {
			t.Fatalf("Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			projectController.CreateOne(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		}
	}

	t.Log("Create a new project with unique name and a description")
	{
		projectOne.Name = "Untitled Project #1"
		projectOne.Description = "a simple job funnel"

		projectOneJson, err := projectOne.ToJson()
		utils.CheckErr(err)
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

				locationHeader := w.Header().Get("Location")
				paths := strings.Split(locationHeader, "/")
				projectOne.ID = paths[1]

				if len(response) < 1 {
					t.Fatalf("\t\t Response payload is empty")
				} else {
					assert.Equal(t, http.StatusCreated, w.Code)
				}
			}
		}
	}

	t.Log("Cannot create project with the same name")
	{
		projectOneJson, err := projectOne.ToJson()
		utils.CheckErr(err)
		projectOneJsonStr := strings.NewReader(string(projectOneJson))

		if req, err := http.NewRequest("POST", "/projects", projectOneJsonStr); err != nil {
			t.Fatalf("\t\t Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			projectController.CreateOne(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		}
	}
}

func TestProjectController_UpdateOne(t *testing.T) {
	pool := tests.GetTestPool()
	projectController := controllers.ProjectController{ Pool: pool }

	t.Log("Cannot update name of project to name of an existing project")
	{
		projectTwo.Name = "Untitled Project #2"
		projectTwo.Description = "untitled project two description"

		projectTwoManager := projectTwo.ToManager()
		projectTwoID, err := projectTwoManager.CreateOne(pool)
		if err != nil {
			utils.CheckErr(err)
		}
		projectTwo.ID = projectTwoID

		projectOne.Name = "Untitled Project #1"
		projectOneJson, err := projectOne.ToJson()
		utils.CheckErr(err)
		projectOneJsonStr := strings.NewReader(string(projectOneJson))

		if req, err := http.NewRequest("PUT", "/projects/"+projectOne.ID, projectOneJsonStr); err != nil {
			t.Fatalf("\t\t request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			router := mux.NewRouter()
			router.HandleFunc("/projects/{id}", projectController.UpdateOne)
			router.ServeHTTP(w, req)

			body, err := ioutil.ReadAll(w.Body)
			if err != nil {
				t.Fatalf("\t\t %v", err.Error())
			}

			fmt.Println(string(body))
			assert.Equal(t, w.Code, http.StatusBadRequest)
		}
	}

	t.Log("Update name and description of an existing project")
	{
		projectTwo.Name = "Project #2"
		projectTwoJson, err := projectTwo.ToJson()
		utils.CheckErr(err)
		projectTwoJsonStr := strings.NewReader(string(projectTwoJson))

		if req, err := http.NewRequest("PUT", "/projects/"+projectTwo.ID, projectTwoJsonStr); err != nil {
			t.Fatalf("\t\t Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			router := mux.NewRouter()
			router.HandleFunc("/projects/{id}", projectController.UpdateOne)
			router.ServeHTTP(w, req)

			body, err := ioutil.ReadAll(w.Body)
			if err != nil {
				t.Fatalf("\t\t %v", err.Error())
			}

			fmt.Println(string(body))

			assert.Equal(t, w.Code, http.StatusOK)
		}
	}
}

func TestProjectController_GetAll(t *testing.T) {
	pool := tests.GetTestPool()
	projectController := controllers.ProjectController{ Pool: pool }

	t.Log("Get all projects with the same name or description")
	{
		if req, err := http.NewRequest("GET", "/projects?limit=10&offset=0", nil); err != nil {
			t.Fatalf("\t\t Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			projectController.GetAll(w, req)
			if _, err := ioutil.ReadAll(w.Body); err != nil {
				utils.CheckErr(err)
			} else {
				assert.Equal(t, w.Code,  http.StatusOK)
			}
		}
	}
}

func TestProjectController_DeleteOne(t *testing.T) {
	pool := tests.GetTestPool()
	projectController := controllers.ProjectController{ Pool: pool }
	projectOneJobOne.ProjectID = projectOne.ID
	projectOneJobOne.Description = "sample job"
	projectOneJobOne.CronSpec = "* * * * *"
	projectOneJobOne.StartDate =  time.Now().Add(60 * time.Second).UTC().Format(time.RFC3339)
	projectOneJobOne.CallbackUrl = "https://time.com"

	manager, err := projectOneJobOne.ToManager()
	utils.CheckErr(err)

	t.Log("Do not delete projects with jobs ")
	{
		_, err = manager.CreateOne(pool)
		utils.CheckErr(err)

		if req, err := http.NewRequest("DELETE", "/projects/"+projectOne.ID, nil); err != nil {
			t.Fatalf("\t\t Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			router := mux.NewRouter()
			router.HandleFunc("/projects/{id}", projectController.DeleteOne)
			router.ServeHTTP(w, req)

			assert.Equal(t, w.Code, http.StatusBadRequest)
		}
	}

	t.Log("Delete project without job")
	{
		if req, err := http.NewRequest("DELETE", "/projects/"+projectTwo.ID, nil); err != nil {
			t.Fatalf("\t\t Request failed %v", err)
		} else {
			w := httptest.NewRecorder()
			router := mux.NewRouter()
			router.HandleFunc("/projects/{id}", projectController.DeleteOne)
			router.ServeHTTP(w, req)

			assert.Equal(t, w.Code, http.StatusNoContent)
		}
	}

}
