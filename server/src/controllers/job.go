package controllers

import (
	"cron-server/server/src/service"
	"cron-server/server/src/transformers"
	"cron-server/server/src/utils"
	"net/http"
	"strconv"
)

type JobController Controller

func (jobController *JobController) ListJobs(w http.ResponseWriter, r *http.Request) {
	projectID, err := utils.ValidateQueryString("projectID", r)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
	}

	limitParam, err := utils.ValidateQueryString("limit", r)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
	}

	offsetParam, err  := utils.ValidateQueryString("offset", r)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
	}

	jobService := service.JobService{
		Pool: jobController.Pool,
		Ctx: r.Context(),
	}

	offset, err := strconv.Atoi(offsetParam)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
	}

	limit, err := strconv.Atoi(limitParam)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
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
	}

	jobService := service.JobService{
		Pool: jobController.Pool,
		Ctx: r.Context(),
	}

	job, err := jobService.CreateJob(jobBody)
	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
	}

	utils.SendJson(w, job, true, http.StatusCreated, nil)

}