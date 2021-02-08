package job

import (
	"github.com/gorilla/mux"
	"github.com/victorlenerd/scheduler0/server/src/controllers"
	"github.com/victorlenerd/scheduler0/server/src/service"
	"github.com/victorlenerd/scheduler0/server/src/transformers"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"net/http"
	"strconv"
)

type JobController controllers.Controller

func (jobController *JobController) ListJobs(w http.ResponseWriter, r *http.Request) {
	projectUUID, err := utils.ValidateQueryString("projectUUID", r)
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

	jobs, getJobsByProjectIDError := jobService.GetJobsByProjectUUID(projectUUID, offset, limit, "date_created")
	if getJobsByProjectIDError != nil {
		utils.SendJson(w, getJobsByProjectIDError.Message, false, getJobsByProjectIDError.Type, nil)
		return
	}

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

	job, createJobError := jobService.CreateJob(jobBody)
	if createJobError != nil {
		utils.SendJson(w, createJobError.Message, false, createJobError.Type, nil)
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
		UUID: params["uuid"],
	}

	jobT, getOneJobError := jobService.GetJob(job)
	if getOneJobError != nil {
		utils.SendJson(w, getOneJobError.Message, false, getOneJobError.Type, nil)
		return
	}

	utils.SendJson(w, jobT, true, http.StatusOK, nil)
}

func (jobController *JobController) UpdateJob(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	body := utils.ExtractBody(w, r)
	jobBody := transformers.Job{}
	err := jobBody.FromJson(body)

	jobBody.UUID = params["uuid"]

	if err != nil {
		utils.SendJson(w, err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	jobService := service.JobService{
		Pool: jobController.Pool,
		Ctx: r.Context(),
	}

	jobT, updateOneJobError := jobService.UpdateJob(jobBody)
	if updateOneJobError != nil {
		utils.SendJson(w, updateOneJobError.Message, false, updateOneJobError.Type, nil)
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
		UUID: params["uuid"],
	}

	deleteOneJobError := jobService.DeleteJob(job)
	if deleteOneJobError != nil {
		utils.SendJson(w, deleteOneJobError.Message, false, deleteOneJobError.Type, nil)
		return
	}

	utils.SendJson(w, nil, true, http.StatusNoContent, nil)
}