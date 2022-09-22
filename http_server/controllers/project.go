package controllers

import (
	"errors"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"scheduler0/models"
	"scheduler0/service"
	"scheduler0/utils"
	"strconv"
)

type projectController struct {
	projectService service.Project
}

type ProjectHTTPController interface {
	CreateOneProject(w http.ResponseWriter, r *http.Request)
	GetOneProject(w http.ResponseWriter, r *http.Request)
	ListProjects(w http.ResponseWriter, r *http.Request)
	DeleteOneProject(w http.ResponseWriter, r *http.Request)
	UpdateOneProject(w http.ResponseWriter, r *http.Request)
}

func NewProjectController(projectService service.Project) ProjectHTTPController {
	return &projectController{projectService: projectService}
}

func (controller *projectController) CreateOneProject(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	utils.CheckErr(err)

	project := models.ProjectModel{}
	err = project.FromJSON(body)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	if len(project.Name) < 1 {
		utils.SendJSON(w, "project name is required", false, http.StatusBadRequest, nil)
		return
	}

	projectTransformer, createOneError := controller.projectService.CreateOne(project)
	if createOneError != nil {
		utils.SendJSON(w, createOneError, false, http.StatusBadRequest, nil)
		return
	}

	customHeader := map[string]string{}

	utils.SendJSON(w, projectTransformer, true, http.StatusCreated, customHeader)
}

func (controller *projectController) GetOneProject(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectId, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.SendJSON(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
	}

	project := models.ProjectModel{
		ID: int64(projectId),
	}

	projectData, err := controller.projectService.GetOneByUUID(project)
	if err != nil {
		utils.SendJSON(w, err.Message, false, err.Type, nil)
		return
	}

	utils.SendJSON(w, projectData, true, http.StatusOK, nil)
}

func (controller *projectController) ListProjects(w http.ResponseWriter, r *http.Request) {
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

	projects, listError := controller.projectService.List(int64(offset), int64(limit))
	if listError != nil {
		utils.SendJSON(w, listError.Message, false, listError.Type, nil)
		return
	}

	utils.SendJSON(w, projects, true, http.StatusOK, nil)
}

func (controller *projectController) DeleteOneProject(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectId, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.SendJSON(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
	}

	project := models.ProjectModel{
		ID: int64(projectId),
	}

	err := controller.projectService.DeleteOneByUUID(project)
	if err != nil {
		utils.SendJSON(w, err.Message, false, err.Type, nil)
		return
	}

	utils.SendJSON(w, nil, true, http.StatusNoContent, nil)
}

func (controller *projectController) UpdateOneProject(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectId, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.SendJSON(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
	}

	body, err := ioutil.ReadAll(r.Body)
	utils.CheckErr(err)

	project := models.ProjectModel{}

	err = project.FromJSON(body)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	project.ID = int64(projectId)

	projectWithSimilarName := models.ProjectModel{
		Name: project.Name,
	}

	projectT, getOneError := controller.projectService.GetOneByName(projectWithSimilarName)
	if getOneError != nil && getOneError.Type != http.StatusNotFound {
		utils.SendJSON(w, getOneError.Message, false, getOneError.Type, nil)
		return
	}

	if projectT != nil {
		utils.SendJSON(w, errors.New("a project with a similar name exists"), false, http.StatusBadRequest, nil)
		return
	}

	updateError := controller.projectService.UpdateOneByUUID(project)
	if updateError != nil {
		utils.SendJSON(w, updateError.Message, false, updateError.Type, nil)
		return
	}

	utils.SendJSON(w, project, true, http.StatusOK, nil)
}
