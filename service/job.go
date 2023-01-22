package service

import (
	"context"
	"fmt"
	"github.com/robfig/cron"
	"log"
	"net/http"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/service/queue"
	"scheduler0/utils"
	"scheduler0/utils/workers"
	"time"
)

type jobService struct {
	jobRepo     repository.Job
	projectRepo repository.Project
	Queue       *queue.JobQueue
	Ctx         context.Context
	logger      *log.Logger
	dispatcher  *workers.Dispatcher
}

type Job interface {
	GetJobsByProjectID(projectID uint64, offset uint64, limit uint64, orderBy string) (*models.PaginatedJob, *utils.GenericError)
	GetJob(job models.JobModel) (*models.JobModel, *utils.GenericError)
	BatchInsertJobs(jobs []models.JobModel) ([]models.JobModel, *utils.GenericError)
	UpdateJob(job models.JobModel) (*models.JobModel, *utils.GenericError)
	DeleteJob(job models.JobModel) *utils.GenericError
	QueueJobs(jobs []models.JobModel)
}

func NewJobService(context context.Context, logger *log.Logger, jobRepo repository.Job, queue *queue.JobQueue, projectRepo repository.Project, dispatcher *workers.Dispatcher) Job {
	service := &jobService{
		jobRepo:     jobRepo,
		projectRepo: projectRepo,
		Queue:       queue,
		Ctx:         context,
		logger:      logger,
		dispatcher:  dispatcher,
	}

	return service
}

// GetJobsByProjectID returns a paginated set of jobs for a project
func (jobService *jobService) GetJobsByProjectID(projectID uint64, offset uint64, limit uint64, orderBy string) (*models.PaginatedJob, *utils.GenericError) {
	count, getCountError := jobService.jobRepo.GetJobsTotalCountByProjectID(projectID)
	if getCountError != nil {
		return nil, getCountError
	}

	if uint64(count) < offset {
		offset = uint64(count)
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

// BatchInsertJobs creates jobs in batches
func (jobService *jobService) BatchInsertJobs(jobs []models.JobModel) ([]models.JobModel, *utils.GenericError) {

	if len(jobs) < 1 {
		return []models.JobModel{}, nil
	}

	for _, job := range jobs {
		if job.Spec != "" {
			if _, err := cron.Parse(job.Spec); err != nil {
				return nil, utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job spec is not valid %s", job.Spec))
			}
		} else {
			return nil, utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job spec is not valid %s", job.Spec))
		}
	}

	projectIds := []uint64{}

	for _, job := range jobs {
		projectIds = append(projectIds, job.ProjectID)
	}

	projects, err := jobService.projectRepo.GetBatchProjectsByIDs(projectIds)
	if err != nil {
		return nil, err
	}

	for _, job := range jobs {
		found := false
		for _, project := range projects {
			if project.ID == job.ProjectID {
				found = true
				break
			}
		}
		if !found {
			return nil, utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("a project in the payload does not exitst. project id %v", job.ProjectID))
		}
	}

	var successChannel = make(chan any)
	var errorChannel = make(chan any)

	jobService.dispatcher.Queue(func(successChannel chan any, errorChannel chan any) {
		insertedIds, err := jobService.jobRepo.BatchInsertJobs(jobs)
		if err != nil {
			errorChannel <- utils.HTTPGenericError(http.StatusInternalServerError, fmt.Sprintf("failed to batch insert job repository: %v", err.Message))
		}

		schedulerTime := utils.GetSchedulerTime()
		now := schedulerTime.GetTime(time.Now())

		for i, insertedId := range insertedIds {
			jobs[i].ID = insertedId
			jobs[i].DateCreated = now
			jobs[i].LastExecutionDate = now
		}

		jobService.QueueJobs(jobs)

		successChannel <- jobs
	}, successChannel, errorChannel)

	for {
		select {
		case res := <-successChannel:
			jobs := res.([]models.JobModel)
			return jobs, nil
		case res := <-errorChannel:
			errM := res.(utils.GenericError)
			return nil, &errM
		}
	}
}

// UpdateJob updates job with ID in transformer. Note that cron expression of job cannot be updated.
func (jobService *jobService) UpdateJob(job models.JobModel) (*models.JobModel, *utils.GenericError) {
	currentJobState := models.JobModel{
		ID: job.ID,
	}
	getErr := jobService.jobRepo.GetOneByID(&currentJobState)
	if getErr != nil {
		return nil, getErr
	}
	if job.Data != "" {
		currentJobState.Data = job.Data
	}
	if job.CallbackUrl != "" {
		currentJobState.CallbackUrl = job.CallbackUrl
	}
	if job.ExecutionType != "" {
		currentJobState.ExecutionType = job.ExecutionType
	}
	_, jobMangerUpdateOneError := jobService.jobRepo.UpdateOneByID(currentJobState)
	if jobMangerUpdateOneError != nil {
		return nil, jobMangerUpdateOneError
	}

	getErr = jobService.jobRepo.GetOneByID(&currentJobState)
	if getErr != nil {
		return nil, getErr
	}

	return &currentJobState, nil
}

// DeleteJob deletes a job with ID in transformer
func (jobService *jobService) DeleteJob(job models.JobModel) *utils.GenericError {
	err := jobService.jobRepo.GetOneByID(&job)
	if err != nil {
		return err
	}

	count, delError := jobService.jobRepo.DeleteOneByID(job)
	if delError != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, delError.Message)
	}

	if count < 1 {
		return utils.HTTPGenericError(http.StatusInternalServerError, "could not find and delete job")
	}

	return nil
}

func (jobService *jobService) QueueJobs(jobs []models.JobModel) {
	jobService.Queue.Queue(jobs)
}
