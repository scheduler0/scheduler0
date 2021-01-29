package controllers

import (
	"errors"
	"github.com/gorilla/mux"
	"github.com/victorlenerd/scheduler0/server/src/service"
	"github.com/victorlenerd/scheduler0/server/src/transformers"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"io/ioutil"
	"net/http"
	"strconv"
)

type ProjectController Controller

func (controller *ProjectController) CreateOne(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	utils.CheckErr(err)

	project := transformers.Project{}
	err = project.FromJson(body)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	if len(project.Name) < 1 {
		utils.SendJson(w, "project name is required", false, http.StatusBadRequest, nil)
		return
	}

	projectService := service.ProjectService{
		Pool: controller.Pool,
	}

	id, err := projectService.CreateOne(project)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	customHeader := map[string]string{}
	customHeader["Location"] = "projects/" + id

	utils.SendJson(w, project, true, http.StatusCreated, customHeader)
}

func (controller *ProjectController) GetOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectID := params["id"]

	if projectID == "" {
		utils.SendJson(w, errors.New("project id is required"), false, http.StatusBadRequest, nil)
		return
	}

	project := transformers.Project{
		ID: projectID,
	}

	projectService := service.ProjectService{
		Pool: controller.Pool,
	}

	projectData, err := projectService.GetOne(project)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.SendJson(w, projectData, true, http.StatusOK, nil)
}

func (controller *ProjectController) GetAll(w http.ResponseWriter, r *http.Request) {
	projectService := service.ProjectService{
		Pool: controller.Pool,
	}

	limitParam, err := utils.ValidateQueryString("limit", r)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offsetParam, err  := utils.ValidateQueryString("offset", r)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offset, err := strconv.Atoi(offsetParam)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	limit, err := strconv.Atoi(limitParam)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	projects, err := projectService.List(offset, limit)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.SendJson(w, projects, true, http.StatusOK, nil)
}

func (controller *ProjectController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectID := params["id"]

	if projectID == "" {
		utils.SendJson(w, errors.New("project id is required"), false, http.StatusBadRequest, nil)
		return
	}

	projectService := service.ProjectService{
		Pool: controller.Pool,
	}

	project := transformers.Project{
		ID: projectID,
	}

	err := projectService.DeleteOne(project)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.SendJson(w, nil, true, http.StatusNoContent, nil)
}

func (controller *ProjectController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectID := params["id"]

	if projectID == "" {
		utils.SendJson(w, errors.New("project id is required"), false, http.StatusBadRequest, nil)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	utils.CheckErr(err)

	project := transformers.Project{}

	err = project.FromJson(body)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	project.ID = projectID

	projectService := service.ProjectService{
		Pool: controller.Pool,
	}

	projectWithSimilarName := transformers.Project{
		Name:  project.Name,
	}

	projectT, _ := projectService.GetOne(projectWithSimilarName)

	if projectT != nil {
		utils.SendJson(w, errors.New("a project with a similar name exists"), false, http.StatusBadRequest, nil)
		return
	}

	err = projectService.UpdateOne(project)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.SendJson(w, project, true, http.StatusOK, nil)
}

