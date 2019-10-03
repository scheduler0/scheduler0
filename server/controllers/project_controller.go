package controllers

import (
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/repository"
	"io/ioutil"
	"net/http"
)

type ProjectController struct {
	Pool repository.Pool
}

var basicProjectController = BasicController{model: models.Project{}}

func (controller *ProjectController) CreateOne(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	misc.CheckErr(err)

	var project models.Project
	project.FromJson(body)

	if len(project.Name) < 1 {
		misc.SendJson(w, "project name is required", false, http.StatusBadRequest, nil)
		return
	}

	id, err := project.CreateOne(&controller.Pool, r.Context())
	if err != nil {
		misc.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	customHeader := map[string]string{}
	customHeader["Location"] = "projects/" + id
	misc.SendJson(w, id, true, http.StatusCreated, nil)
}

func (controller *ProjectController) GetOne(w http.ResponseWriter, r *http.Request) {
	basicProjectController.GetOne(w, r, controller.Pool)
}

func (controller *ProjectController) GetAll(w http.ResponseWriter, r *http.Request) {
	basicProjectController.GetAll(w, r, controller.Pool)
}

func (controller *ProjectController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	basicProjectController.DeleteOne(w, r, controller.Pool)
}

func (controller *ProjectController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	basicProjectController.UpdateOne(w, r, controller.Pool)
}

func (controller *ProjectController) GetAllOrCreateOne(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		controller.GetAll(w, r)
	}

	if r.Method == http.MethodPost {
		controller.CreateOne(w, r)
	}
}
