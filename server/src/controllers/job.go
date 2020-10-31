package controllers

import (
	"cron-server/server/src/service"
	"cron-server/server/src/transformers"
	"cron-server/server/src/utils"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
)

type JobController Controller

func (jobController *JobController) ListJobs(w http.ResponseWriter, r *http.Request) {
	projectID, err := utils.ValidateQueryString("projectID", r)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
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

	jobService := service.JobService{
		Pool: jobController.Pool,
		Ctx: r.Context(),
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

	jobs, err := jobService.GetJobsByProjectID(projectID, offset, limit, "date_created")

	utils.SendJson(w, jobs, true, http.StatusOK, nil)
}

func (jobController *JobController) CreateJob(w http.ResponseWriter, r *http.Request) {
	body := utils.ExtractBody(w, r)
	jobBody := transformers.Job{}
	err := jobBody.FromJson(body)

	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	jobService := service.JobService{
		Pool: jobController.Pool,
		Ctx: r.Context(),
	}

	job, err := jobService.CreateJob(jobBody)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.SendJson(w, job, true, http.StatusCreated, nil)
}

func (jobController *JobController) GetAJob(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	jobService := service.JobService{
		Pool: jobController.Pool,
		Ctx: r.Context(),
	}

	job := transformers.Job{
		ID: params["id"],
	}

	jobT, err := jobService.GetJob(job)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.SendJson(w, jobT, true, http.StatusOK, nil)
}

func (jobController *JobController) UpdateJob(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	body := utils.ExtractBody(w, r)
	jobBody := transformers.Job{}
	err := jobBody.FromJson(body)

	jobBody.ID = params["id"]

	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	jobService := service.JobService{
		Pool: jobController.Pool,
		Ctx: r.Context(),
	}

	jobT, err := jobService.UpdateJob(jobBody)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.SendJson(w, jobT, true, http.StatusOK, nil)
}

func (jobController *JobController) DeleteJob(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	jobService := service.JobService{
		Pool: jobController.Pool,
		Ctx: r.Context(),
	}

	job := transformers.Job{
		ID: params["id"],
	}

	err := jobService.DeleteJob(job)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.SendJson(w, nil, true, http.StatusNoContent, nil)
}