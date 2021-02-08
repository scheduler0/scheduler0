package service

import (
	"github.com/victorlenerd/scheduler0/server/src/managers/job"
	"github.com/victorlenerd/scheduler0/server/src/transformers"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"net/http"
)

type JobService Service

func (jobService *JobService) GetJobsByProjectUUID(projectUUID string, offset int, limit int, orderBy string) ([]transformers.Job, *utils.GenericError) {
	jobManager := job.JobManager{}

	jobManagers, err := jobManager.GetAll(jobService.Pool, projectUUID, offset, limit, orderBy)
	if err != nil {
		return nil, err
	}

	jobs := make([]transformers.Job, 0, len(jobManagers))

	for _, jobManager := range jobManagers {
		jobsTransformer := transformers.Job{}
		jobsTransformer.FromManager(jobManager)
		jobs = append(jobs, jobsTransformer)
	}

	return jobs, nil
}

func (jobService *JobService) GetJob(job transformers.Job) (*transformers.Job, *utils.GenericError) {
	jobManager, err := job.ToManager()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	jobMangerGetOneError := jobManager.GetOne(jobService.Pool, job.UUID)
	if jobMangerGetOneError != nil {
		return nil, jobMangerGetOneError
	}

	job.FromManager(jobManager)

	return &job, nil
}

func (jobService *JobService) CreateJob(job transformers.Job) (*transformers.Job, *utils.GenericError) {
	jobManager, err := job.ToManager()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	_, jobMangerCreateOneError := jobManager.CreateOne(jobService.Pool)
	if jobMangerCreateOneError != nil {
		return nil, jobMangerCreateOneError
	}

	job.FromManager(jobManager)

	// TODO: Start go routine for job

	return &job, nil
}

func (jobService *JobService) UpdateJob(job transformers.Job) (*transformers.Job, *utils.GenericError) {
	jobManager, err := job.ToManager()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	_, jobMangerUpdateOneError := jobManager.UpdateOne(jobService.Pool)
	if jobMangerUpdateOneError != nil {
		return nil, jobMangerUpdateOneError
	}

	job.FromManager(jobManager)

	return &job, nil
}

func (jobService *JobService) DeleteJob(jobTransformer transformers.Job) *utils.GenericError {
	jobManager := job.JobManager{
		UUID: jobTransformer.UUID,
	}

	err := jobManager.GetOne(jobService.Pool, jobManager.UUID)
	if err != nil {
		return err
	}

	count, delError := jobManager.DeleteOne(jobService.Pool)
	if delError != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, delError.Message)
	}

	if count < 1 {
		return utils.HTTPGenericError(http.StatusInternalServerError, "could not find and delete job")
	}

	return nil
}



