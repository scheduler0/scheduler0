package controllers

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"scheduler0/models"
	"scheduler0/service"
	"scheduler0/utils"
	"strconv"
)

// HTTPController http request handler for /job requests
type jobHTTPController struct {
	jobService service.Job
	logger     *log.Logger
}

type JobHTTPController interface {
	ListJobs(w http.ResponseWriter, r *http.Request)
	BatchCreateJobs(w http.ResponseWriter, r *http.Request)
	GetOneJob(w http.ResponseWriter, r *http.Request)
	UpdateOneJob(w http.ResponseWriter, r *http.Request)
	DeleteOneJob(w http.ResponseWriter, r *http.Request)
}

func NewJoBHTTPController(logger *log.Logger, jobService service.Job) JobHTTPController {
	return &jobHTTPController{
		jobService: jobService,
		logger:     logger,
	}
}

// ListJobs returns a paginated list of jobs
func (jobController *jobHTTPController) ListJobs(w http.ResponseWriter, r *http.Request) {
	projectIDQueryParam, err := utils.ValidateQueryString("projectId", r)
	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	projectID, convertErr := strconv.Atoi(projectIDQueryParam)
	if convertErr != nil {
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
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

	jobs, getJobsByProjectIDError := jobController.jobService.GetJobsByProjectID(int64(projectID), int64(offset), int64(limit), "date_created")
	if getJobsByProjectIDError != nil {
		utils.SendJSON(w, getJobsByProjectIDError.Message, false, getJobsByProjectIDError.Type, nil)
		return
	}

	utils.SendJSON(w, jobs, true, http.StatusOK, nil)
}

// BatchCreateJobs handles request to job in batches
func (jobController *jobHTTPController) BatchCreateJobs(w http.ResponseWriter, r *http.Request) {
	body := utils.ExtractBody(w, r)

	if body == nil {
		return
	}

	jobTransformers := &[]models.JobModel{}
	if err := json.Unmarshal(body, jobTransformers); err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusUnprocessableEntity, nil)
		return
	}

	createJobTransformers, batchCreateError := jobController.jobService.BatchInsertJobs(*jobTransformers)
	if batchCreateError != nil {
		jobController.logger.Println("batchCreateError", batchCreateError.Message)
		utils.SendJSON(w, batchCreateError.Message, false, batchCreateError.Type, nil)
		return
	}

	go jobController.jobService.QueueJobs(createJobTransformers)

	utils.SendJSON(w, createJobTransformers, true, http.StatusCreated, nil)
}

// GetOneJob handles request to return a single job
func (jobController *jobHTTPController) GetOneJob(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	jobID, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}

	job := models.JobModel{
		ID: int64(jobID),
	}

	jobT, getOneJobError := jobController.jobService.GetJob(job)
	if getOneJobError != nil {
		utils.SendJSON(w, getOneJobError.Message, false, getOneJobError.Type, nil)
		return
	}

	utils.SendJSON(w, jobT, true, http.StatusOK, nil)
}

// UpdateOneJob handles request to update a single job
func (jobController *jobHTTPController) UpdateOneJob(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	body := utils.ExtractBody(w, r)
	jobBody := models.JobModel{}
	err := jobBody.FromJSON(body)

	jobID, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}

	jobBody.ID = int64(jobID)

	if err != nil {
		utils.SendJSON(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	jobT, updateOneJobError := jobController.jobService.UpdateJob(jobBody)
	if updateOneJobError != nil {
		utils.SendJSON(w, updateOneJobError.Message, false, updateOneJobError.Type, nil)
		return
	}

	utils.SendJSON(w, jobT, true, http.StatusOK, nil)
}

// DeleteOneJob handles request to delete a single job
func (jobController *jobHTTPController) DeleteOneJob(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	jobID, convertErr := strconv.Atoi(params["id"])
	if convertErr != nil {
		utils.SendJSON(w, convertErr.Error(), false, http.StatusBadRequest, nil)
		return
	}

	job := models.JobModel{
		ID: int64(jobID),
	}

	deleteOneJobError := jobController.jobService.DeleteJob(job)
	if deleteOneJobError != nil {
		utils.SendJSON(w, deleteOneJobError.Message, false, deleteOneJobError.Type, nil)
		return
	}

	utils.SendJSON(w, nil, true, http.StatusNoContent, nil)
}
