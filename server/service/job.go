package service

import (
	"fmt"
	"net/http"
	"scheduler0/server/managers/job"
	"scheduler0/server/transformers"
	"scheduler0/utils"
)

// JobService handles the business logic for jobs
type JobService Service

// GetJobsByProjectUUID returns a paginated set of jobs for a project
func (jobService *JobService) GetJobsByProjectUUID(projectUUID string, offset int, limit int, orderBy string) (*transformers.PaginatedJob, *utils.GenericError) {
	jobManager := job.Manager{}

	count, getCountError := jobManager.GetJobsTotalCountByProjectUUID(jobService.DBConnection, projectUUID)
	if getCountError != nil {
		return nil, getCountError
	}

	if count < offset {
		return nil, utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("there are %v jobs which is less than %v", count, offset))
	}

	jobManagers, err := jobManager.GetAll(jobService.DBConnection, projectUUID, offset, limit, orderBy)
	if err != nil {
		return nil, err
	}

	jobs := make([]transformers.Job, 0, len(jobManagers))

	for _, jobManager := range jobManagers {
		jobsTransformer := transformers.Job{}
		jobsTransformer.FromManager(jobManager)
		jobs = append(jobs, jobsTransformer)
	}

	paginatedJobs := transformers.PaginatedJob{}
	paginatedJobs.Data = jobs
	paginatedJobs.Limit = limit
	paginatedJobs.Total = count
	paginatedJobs.Offset = offset

	return &paginatedJobs, nil
}

// GetJob returns a job with UUID that matched UUID of transformer
func (jobService *JobService) GetJob(jobTransformer transformers.Job) (*transformers.Job, *utils.GenericError) {
	jobManager, err := jobTransformer.ToManager()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	jobMangerGetOneError := jobManager.GetOne(jobService.DBConnection, jobTransformer.UUID)
	if jobMangerGetOneError != nil {
		return nil, jobMangerGetOneError
	}

	jobTransformer.FromManager(jobManager)

	return &jobTransformer, nil
}

// CreateJob creates a new job based on values in transformer object
func (jobService *JobService) CreateJob(jobTransformer transformers.Job) (*transformers.Job, *utils.GenericError) {
	jobManager, err := jobTransformer.ToManager()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	_, jobMangerCreateOneError := jobManager.CreateOne(jobService.DBConnection)
	if jobMangerCreateOneError != nil {
		return nil, jobMangerCreateOneError
	}

	jobTransformer.FromManager(jobManager)

	return &jobTransformer, nil
}

// UpdateJob updates job with UUID in transformer. Note that cron expression of job cannot be updated.
func (jobService *JobService) UpdateJob(jobTransformer transformers.Job) (*transformers.Job, *utils.GenericError) {
	jobManager, err := jobTransformer.ToManager()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	_, jobMangerUpdateOneError := jobManager.UpdateOne(jobService.DBConnection)
	if jobMangerUpdateOneError != nil {
		return nil, jobMangerUpdateOneError
	}

	jobTransformer.FromManager(jobManager)

	return &jobTransformer, nil
}

// DeleteJob deletes a job with UUID in transformer
func (jobService *JobService) DeleteJob(jobTransformer transformers.Job) *utils.GenericError {
	jobManager := job.Manager{
		UUID: jobTransformer.UUID,
	}

	err := jobManager.GetOne(jobService.DBConnection, jobManager.UUID)
	if err != nil {
		return err
	}

	count, delError := jobManager.DeleteOne(jobService.DBConnection)
	if delError != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, delError.Message)
	}

	if count < 1 {
		return utils.HTTPGenericError(http.StatusInternalServerError, "could not find and delete job")
	}

	return nil
}
