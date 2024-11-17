package controllers

import (
	"errors"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"scheduler0/pkg/models"
	"scheduler0/pkg/service/project"
	"scheduler0/pkg/utils"
	"strconv"
)

type projectController struct {
	projectService project.ProjectService
	logger         *log.Logger
}

type ProjectHTTPController interface {
	CreateOneProject(w http.ResponseWriter, r *http.Request)
	GetOneProject(w http.ResponseWriter, r *http.Request)
	ListProjects(w http.ResponseWriter, r *http.Request)
	DeleteOneProject(w http.ResponseWriter, r *http.Request)
	UpdateOneProject(w http.ResponseWriter, r *http.Request)
}

func NewProjectController(logger *log.Logger, projectService project.ProjectService) ProjectHTTPController {
	return &projectController{
		projectService: projectService,
		logger:         logger,
	}
}

func (controller *projectController) CreateOneProject(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		controller.logger.Fatalln(err)
	}

	project := models.Project{}
	err = project.FromJSON(body)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	projectTransformer, createOneError := controller.projectService.CreateOne(project)
	if createOneError != nil {
		utils.SendJSON(w, createOneError, false, createOneError.Type, nil)
		return
	}

	utils.SendJSON(w, projectTransformer, true, http.StatusCreated, nil)
}

func (controller *projectController) GetOneProject(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	projectId, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.SendJSON(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
		return
	}

	project := models.Project{
		ID: uint64(projectId),
	}

	err := controller.projectService.GetOneByID(&project)
	if err != nil {
		utils.SendJSON(w, err.Message, false, err.Type, nil)
		return
	}

	utils.SendJSON(w, project, true, http.StatusOK, nil)
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

	projects, listError := controller.projectService.List(uint64(offset), uint64(limit))
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
		return
	}

	project := models.Project{
		ID: uint64(projectId),
	}

	err := controller.projectService.DeleteOneByID(project)
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
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		controller.logger.Fatalln(err)
	}
	project := models.Project{}

	err = project.FromJSON(body)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	project.ID = uint64(projectId)

	updateError := controller.projectService.UpdateOneByID(&project)
	if updateError != nil {
		utils.SendJSON(w, updateError.Message, false, updateError.Type, nil)
		return
	}

	utils.SendJSON(w, project, true, http.StatusOK, nil)
}
