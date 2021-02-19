package project

import (
	"errors"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"scheduler0/server/src/controllers"
	"scheduler0/server/src/service"
	"scheduler0/server/src/transformers"
	"scheduler0/server/src/utils"
	"strconv"
)

type ProjectController controllers.Controller

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

	id, createOneError := projectService.CreateOne(project)
	if createOneError != nil {
		utils.SendJson(w, createOneError, false, http.StatusBadRequest, nil)
		return
	}

	customHeader := map[string]string{}
	customHeader["Location"] = "projects/" + id

	utils.SendJson(w, project, true, http.StatusCreated, customHeader)
}

func (controller *ProjectController) GetOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectUUID := params["uuid"]

	if projectUUID == "" {
		utils.SendJson(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
		return
	}

	project := transformers.Project{
		UUID: projectUUID,
	}

	projectService := service.ProjectService{
		Pool: controller.Pool,
	}

	projectData, err := projectService.GetOneByUUID(project)
	if err != nil {
		utils.SendJson(w, err.Message, false, err.Type, nil)
		return
	}

	utils.SendJson(w, projectData, true, http.StatusOK, nil)
}

func (controller *ProjectController) List(w http.ResponseWriter, r *http.Request) {
	projectService := service.ProjectService{
		Pool: controller.Pool,
	}

	limitParam, err := utils.ValidateQueryString("limit", r)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offsetParam, err := utils.ValidateQueryString("offset", r)
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

	projects, listError := projectService.List(offset, limit)
	if listError != nil {
		utils.SendJson(w, listError.Message, false, listError.Type, nil)
		return
	}

	utils.SendJson(w, projects, true, http.StatusOK, nil)
}

func (controller *ProjectController) DeleteOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectUUID := params["uuid"]

	if projectUUID == "" {
		utils.SendJson(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
		return
	}

	projectService := service.ProjectService{
		Pool: controller.Pool,
	}

	project := transformers.Project{
		UUID: projectUUID,
	}

	err := projectService.DeleteOne(project)
	if err != nil {
		utils.SendJson(w, err.Message, false, err.Type, nil)
		return
	}

	utils.SendJson(w, nil, true, http.StatusNoContent, nil)
}

func (controller *ProjectController) UpdateOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectUUID := params["uuid"]

	if projectUUID == "" {
		utils.SendJson(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
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

	project.UUID = projectUUID

	projectService := &service.ProjectService{
		Pool: controller.Pool,
	}

	projectWithSimilarName := transformers.Project{
		Name: project.Name,
	}

	projectT, getOneError := projectService.GetOneByName(projectWithSimilarName)
	if getOneError != nil && getOneError.Type != http.StatusNotFound {
		utils.SendJson(w, getOneError.Message, false, getOneError.Type, nil)
		return
	}

	if projectT != nil {
		utils.SendJson(w, errors.New("a project with a similar name exists"), false, http.StatusBadRequest, nil)
		return
	}

	updateError := projectService.UpdateOne(project)
	if updateError != nil {
		utils.SendJson(w, updateError.Message, false, updateError.Type, nil)
		return
	}

	utils.SendJson(w, project, true, http.StatusOK, nil)
}
