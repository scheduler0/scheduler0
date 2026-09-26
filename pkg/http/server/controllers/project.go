package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	"scheduler0/pkg/service/project"
	"scheduler0/pkg/utils"
	"strconv"

	"github.com/gorilla/mux"
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
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneProject entry", r.URL.Path))

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneProject error: failed to read request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}

	project := models.Project{}
	err = project.FromJSON(body)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneProject error: failed to unmarshal request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	if project.CreatedBy == "" {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneProject error: createdBy is required", r.URL.Path))
		utils.SendJSON(w, "createdBy is required", false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		requestID := utils.GetRequestID(r.Context())
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneProject error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	project.AccountId = accountId

	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneProject processing, accountId=%d", r.URL.Path, accountId))

	projectTransformer, createOneError := controller.projectService.CreateOne(project)
	if createOneError != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneProject error: failed to create project, accountId=%d, error=%s", r.URL.Path, accountId, createOneError.Message))
		utils.SendJSON(w, createOneError, false, createOneError.Type, nil)
		return
	}

	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneProject success, status=201, projectId=%d, accountId=%d", r.URL.Path, projectTransformer.ID, accountId))
	utils.SendJSON(w, projectTransformer, true, http.StatusCreated, nil)
}

func (controller *projectController) GetOneProject(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	projectId, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetOneProject error: invalid project ID parameter, id=%s, error=%v", r.URL.Path, params["id"], convertErr))
		utils.SendJSON(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
		return
	}

	project := models.Project{
		ID: uint64(projectId),
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		requestID := utils.GetRequestID(r.Context())
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneProject error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	project.AccountId = accountId

	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetOneProject entry, projectId=%d, accountId=%d", r.URL.Path, projectId, accountId))

	getErr := controller.projectService.GetOneByID(&project)
	if getErr != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetOneProject error: failed to get project, projectId=%d, accountId=%d, error=%v", r.URL.Path, projectId, accountId, getErr))
		utils.SendJSON(w, getErr.Error(), false, getErr.Type, nil)
		return
	}

	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - GetOneProject success, status=200, projectId=%d, accountId=%d", r.URL.Path, projectId, accountId))
	utils.SendJSON(w, project, true, http.StatusOK, nil)
}

func (controller *projectController) ListProjects(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - ListProjects entry, query=%s", r.URL.Path, r.URL.RawQuery))

	defaultLimit := strconv.Itoa(constants.DefaultListLimit)
	defaultOffset := "0"

	limitParam, err := utils.ValidateQueryStringWithDefault("limit", r, &defaultLimit)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - ListProjects error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offsetParam, err := utils.ValidateQueryStringWithDefault("offset", r, &defaultOffset)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - ListProjects error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	offset, err := strconv.Atoi(offsetParam)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - ListProjects error: invalid offset parameter, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	limit, err := strconv.Atoi(limitParam)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - ListProjects error: invalid limit parameter, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	defaultOrderByColumn := constants.ProjectsDateCreatedColumn
	defaultOrderByDirection := constants.OrderDirectionDesc

	orderByColumn, err := utils.ValidateQueryStringWithDefault("orderBy", r, &defaultOrderByColumn)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - ListProjects error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	orderByDirection, err := utils.ValidateQueryStringWithDefault("orderByDirection", r, &defaultOrderByDirection)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - ListProjects error: %v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		requestID := utils.GetRequestID(r.Context())
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneProject error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}

	projects, listError := controller.projectService.List(uint64(offset), uint64(limit), accountId, orderByColumn, orderByDirection)
	if listError != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - ListProjects error: failed to list projects, accountId=%d, offset=%d, limit=%d, error=%s", r.URL.Path, accountId, offset, limit, listError.Message))
		utils.SendJSON(w, listError.Message, false, listError.Type, nil)
		return
	}

	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("GET %s - ListProjects success, status=200, accountId=%d, count=%d, offset=%d, limit=%d", r.URL.Path, accountId, projects.Total, offset, limit))
	utils.SendJSON(w, projects, true, http.StatusOK, nil)
}

func (controller *projectController) DeleteOneProject(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	projectId, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneProject error: invalid project ID parameter, id=%s, error=%v", r.URL.Path, params["id"], convertErr))
		utils.SendJSON(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
		return
	}

	// Parse request body to get deletedBy
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneProject error: failed to read request body, projectId=%d, error=%v", r.URL.Path, projectId, err))
		utils.SendJSON(w, "Failed to read request body", false, http.StatusBadRequest, nil)
		return
	}

	var deleteRequest struct {
		DeletedBy string `json:"deletedBy"`
	}

	if err := json.Unmarshal(body, &deleteRequest); err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneProject error: invalid request body, projectId=%d, error=%v", r.URL.Path, projectId, err))
		utils.SendJSON(w, "Invalid request body", false, http.StatusBadRequest, nil)
		return
	}

	if deleteRequest.DeletedBy == "" {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneProject error: deletedBy is required, projectId=%d", r.URL.Path, projectId))
		utils.SendJSON(w, "deletedBy is required", false, http.StatusBadRequest, nil)
		return
	}

	project := models.Project{
		ID:        uint64(projectId),
		DeletedBy: &deleteRequest.DeletedBy,
	}

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		requestID := utils.GetRequestID(r.Context())
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneProject error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	project.AccountId = accountId

	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneProject entry, projectId=%d, accountId=%d, deletedBy=%s", r.URL.Path, projectId, accountId, deleteRequest.DeletedBy))

	deleteErr := controller.projectService.DeleteOneByID(project)
	if deleteErr != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneProject error: failed to delete project, projectId=%d, accountId=%d, error=%v", r.URL.Path, projectId, accountId, deleteErr))
		utils.SendJSON(w, deleteErr.Error(), false, deleteErr.Type, nil)
		return
	}

	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("DELETE %s - DeleteOneProject success, status=204, projectId=%d, accountId=%d", r.URL.Path, projectId, accountId))
	utils.SendJSON(w, nil, true, http.StatusNoContent, nil)
}

func (controller *projectController) UpdateOneProject(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	params := mux.Vars(r)

	projectId, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneProject error: invalid project ID parameter, id=%s, error=%v", r.URL.Path, params["id"], convertErr))
		utils.SendJSON(w, errors.New("project uuid is required"), false, http.StatusBadRequest, nil)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneProject error: failed to read request body, projectId=%d, error=%v", r.URL.Path, projectId, err))
		utils.SendJSON(w, "request body required", false, http.StatusBadRequest, nil)
		return
	}
	project := models.Project{}

	err = project.FromJSON(body)
	if err != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneProject error: failed to unmarshal request body, projectId=%d, error=%v", r.URL.Path, projectId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	if project.ModifiedBy == nil || *project.ModifiedBy == "" {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneProject error: modifiedBy is required, projectId=%d", r.URL.Path, projectId))
		utils.SendJSON(w, "modifiedBy is required", false, http.StatusBadRequest, nil)
		return
	}

	project.ID = uint64(projectId)

	accountId, ok := utils.GetAccountID(r.Context())
	if !ok {
		requestID := utils.GetRequestID(r.Context())
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("POST %s - CreateOneProject error: account ID not found in context", r.URL.Path))
		utils.SendJSON(w, "account ID not found in context", false, http.StatusInternalServerError, nil)
		return
	}
	project.AccountId = accountId

	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneProject entry, projectId=%d, accountId=%d", r.URL.Path, projectId, accountId))

	updateError := controller.projectService.UpdateOneByID(&project)
	if updateError != nil {
		utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneProject error: failed to update project, projectId=%d, accountId=%d, error=%s", r.URL.Path, projectId, accountId, updateError.Message))
		utils.SendJSON(w, updateError.Message, false, updateError.Type, nil)
		return
	}

	utils.LogWithRequestID(controller.logger, requestID, "", fmt.Sprintf("PUT %s - UpdateOneProject success, status=200, projectId=%d, accountId=%d", r.URL.Path, projectId, accountId))
	utils.SendJSON(w, project, true, http.StatusOK, nil)
}
