package controllers

import (
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/repository"
	"io/ioutil"
	"net/http"
)

type ProjectController struct {
	pool *repository.Pool
}

var basicProjectController = BasicController{model: models.Project{}, pool: repository.Pool{}}

func (controller *ProjectController) CreateOne(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	misc.CheckErr(err)

	var project models.Project

	project.FromJson(body)

	if len(project.Name) < 1 {
		misc.SendJson(w, "Project name is required", http.StatusBadRequest, nil)
		return
	}

	id, err := project.CreateOne(controller.pool)
	if err != nil {
		misc.SendJson(w, err, http.StatusBadRequest, nil)
	}

	customHeader := map[string]string{}
	customHeader["Location"] = "projects/" + id

	misc.SendJson(w, id, http.StatusCreated, nil)
}

func (_ *ProjectController) GetOne(w http.ResponseWriter, r *http.Request) {
	basicProjectController.GetOne(w, r)
}

func (_ *ProjectController) GetAll(w http.ResponseWriter, r *http.Request) {
	basicProjectController.GetAll(w, r)
}

func (_ *ProjectController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	basicProjectController.DeleteOne(w, r)
}

func (_ *ProjectController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	basicProjectController.UpdateOne(w, r)
}

func (controller *ProjectController) GetAllOrCreateOne(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		controller.GetAll(w, r)
	}

	if r.Method == http.MethodPost {
		controller.CreateOne(w, r)
	}
}
