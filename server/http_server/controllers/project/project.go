package project

import (
	"errors"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"scheduler0/server/http_server/controllers"
	"scheduler0/server/service"
	"scheduler0/server/transformers"
	"scheduler0/utils"
	"strconv"
)

type Controller controllers.Controller

func (controller *Controller) CreateOne(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	utils.CheckErr(err)

	project := transformers.Project{}
	err = project.FromJSON(body)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	if len(project.Name) < 1 {
		utils.SendJSON(w, "project name is required", false, http.StatusBadRequest, nil)
		return
	}

	projectService := service.ProjectService{
		Pool: controller.Pool,
	}

	projectTransformer, createOneError := projectService.CreateOne(project)
	if createOneError != nil {
		utils.SendJSON(w, createOneError, false, http.StatusBadRequest, nil)
		return
	}

	customHeader := map[string]string{}

	utils.SendJSON(w, projectTransformer, true, http.StatusCreated, customHeader)
}

func (controller *Controller) GetOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectUUID := params["uuid"]

	if projectUUID == "" {
		utils.SendJSON(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
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
		utils.SendJSON(w, err.Message, false, err.Type, nil)
		return
	}

	utils.SendJSON(w, projectData, true, http.StatusOK, nil)
}

func (controller *Controller) List(w http.ResponseWriter, r *http.Request) {
	projectService := service.ProjectService{
		Pool: controller.Pool,
	}

	limitParam, err := utils.ValidateQueryString("limit", r)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offsetParam, err := utils.ValidateQueryString("offset", r)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offset, err := strconv.Atoi(offsetParam)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	limit, err := strconv.Atoi(limitParam)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	projects, listError := projectService.List(offset, limit)
	if listError != nil {
		utils.SendJSON(w, listError.Message, false, listError.Type, nil)
		return
	}

	utils.SendJSON(w, projects, true, http.StatusOK, nil)
}

func (controller *Controller) DeleteOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectUUID := params["uuid"]

	if projectUUID == "" {
		utils.SendJSON(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
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
		utils.SendJSON(w, err.Message, false, err.Type, nil)
		return
	}

	utils.SendJSON(w, nil, true, http.StatusNoContent, nil)
}

func (controller *Controller) UpdateOne(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectUUID := params["uuid"]

	if projectUUID == "" {
		utils.SendJSON(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	utils.CheckErr(err)

	project := transformers.Project{}

	err = project.FromJSON(body)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
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
		utils.SendJSON(w, getOneError.Message, false, getOneError.Type, nil)
		return
	}

	if projectT != nil {
		utils.SendJSON(w, errors.New("a project with a similar name exists"), false, http.StatusBadRequest, nil)
		return
	}

	updateError := projectService.UpdateOne(project)
	if updateError != nil {
		utils.SendJSON(w, updateError.Message, false, updateError.Type, nil)
		return
	}

	utils.SendJSON(w, project, true, http.StatusOK, nil)
}
