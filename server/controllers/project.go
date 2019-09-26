package controllers

import (
	"cron-server/server/misc"
	"cron-server/server/models"
	"io/ioutil"
	"net/http"
)

type ProjectController struct{}

var basicProjectController = BasicController{model: models.Project{}}

func (_ *ProjectController) CreateOne(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	misc.CheckErr(err)

	var project models.Project

	project.FromJson(body)

	if len(project.Name) < 1 {
		misc.SendJson(w, "Project name is required", http.StatusBadRequest, nil)
		return
	}

	id, err := project.CreateOne()
	misc.CheckErr(err)

	customHeader := map[string]string{}
	customHeader["Location"] = "Projects/" + id

	misc.SendJson(w, id, http.StatusCreated, customHeader)
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
