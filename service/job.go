package service

import (
	"context"
	"fmt"
	"net/http"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/utils"
)

type jobService struct {
	jobRepo repository.Job
	Ctx     context.Context
}

type Job interface {
	GetJobsByProjectID(projectID int64, offset int64, limit int64, orderBy string) (*models.PaginatedJob, *utils.GenericError)
	GetJob(jobTransformer models.JobModel) (*models.JobModel, *utils.GenericError)
	CreateJob(jobTransformer models.JobModel) (*models.JobModel, *utils.GenericError)
	BatchInsertJobs(jobTransformers []models.JobModel) ([]models.JobModel, *utils.GenericError)
	UpdateJob(jobTransformer models.JobModel) (*models.JobModel, *utils.GenericError)
	DeleteJob(jobTransformer models.JobModel) *utils.GenericError
}

func NewJobService(jobRepo repository.Job, context context.Context) Job {
	return &jobService{
		jobRepo: jobRepo,
		Ctx:     context,
	}
}

// GetJobsByProjectID returns a paginated set of jobs for a project
func (jobService *jobService) GetJobsByProjectID(projectID int64, offset int64, limit int64, orderBy string) (*models.PaginatedJob, *utils.GenericError) {
	count, getCountError := jobService.jobRepo.GetJobsTotalCountByProjectID(projectID)
	if getCountError != nil {
		return nil, getCountError
	}

	if count < offset {
		return nil, utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("there are %v jobs which is less than %v", count, offset))
	}

	jobManagers, err := jobService.jobRepo.GetAllByProjectID(projectID, offset, limit, orderBy)
	if err != nil {
		return nil, err
	}

	paginatedJobs := models.PaginatedJob{}
	paginatedJobs.Data = jobManagers
	paginatedJobs.Limit = limit
	paginatedJobs.Total = count
	paginatedJobs.Offset = offset

	return &paginatedJobs, nil
}

// GetJob returns a job with ID that matched ID of transformer
func (jobService *jobService) GetJob(job models.JobModel) (*models.JobModel, *utils.GenericError) {
	jobMangerGetOneError := jobService.jobRepo.GetOneByID(&job)
	if jobMangerGetOneError != nil {
		return nil, jobMangerGetOneError
	}

	return &job, nil
}

// CreateJob creates a new job based on values in transformer object
func (jobService *jobService) CreateJob(job models.JobModel) (*models.JobModel, *utils.GenericError) {
	_, jobMangerCreateOneError := jobService.jobRepo.CreateOne(job)
	if jobMangerCreateOneError != nil {
		return nil, jobMangerCreateOneError
	}

	return &job, nil
}

// BatchInsertJobs creates jobs in batches
func (jobService *jobService) BatchInsertJobs(jobTransformers []models.JobModel) ([]models.JobModel, *utils.GenericError) {
	insertedIds, err := jobService.jobRepo.BatchInsertJobs(jobTransformers)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, fmt.Sprintf("failed to batch insert job repository: %v", err.Message))
	}

	for i, insertedId := range insertedIds {
		jobTransformers[i].ID = insertedId
	}

	return jobTransformers, nil
}

// UpdateJob updates job with ID in transformer. Note that cron expression of job cannot be updated.
func (jobService *jobService) UpdateJob(jobTransformer models.JobModel) (*models.JobModel, *utils.GenericError) {
	_, jobMangerUpdateOneError := jobService.jobRepo.UpdateOneByID(jobTransformer)
	if jobMangerUpdateOneError != nil {
		return nil, jobMangerUpdateOneError
	}

	return &jobTransformer, nil
}

// DeleteJob deletes a job with ID in transformer
func (jobService *jobService) DeleteJob(jobTransformer models.JobModel) *utils.GenericError {
	err := jobService.jobRepo.GetOneByID(&jobTransformer)
	if err != nil {
		return err
	}

	count, delError := jobService.jobRepo.DeleteOneByID(jobTransformer)
	if delError != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, delError.Message)
	}

	if count < 1 {
		return utils.HTTPGenericError(http.StatusInternalServerError, "could not find and delete job")
	}

	return nil
}
