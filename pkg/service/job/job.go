package job

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/robfig/cron"
	"net/http"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	"scheduler0/pkg/repository/job"
	"scheduler0/pkg/repository/project"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/service/async_task"
	"scheduler0/pkg/service/queue"
	"scheduler0/pkg/utils"
	"time"
)

type jobService struct {
	jobRepo          job.JobRepo
	projectRepo      project.ProjectRepo
	Queue            queue.JobQueueService
	Ctx              context.Context
	logger           hclog.Logger
	dispatcher       *utils.Dispatcher
	asyncTaskManager async_task.AsyncTaskService
}

//go:generate mockery --name JobService --output ../mocks
type JobService interface {
	GetJobsByProjectID(projectID uint64, offset uint64, limit uint64, orderBy string) (*models.PaginatedJob, *utils.GenericError)
	GetJob(job models.Job) (*models.Job, *utils.GenericError)
	BatchInsertJobs(requestId string, jobs []models.Job) ([]uint64, *utils.GenericError)
	UpdateJob(job models.Job) (*models.Job, *utils.GenericError)
	DeleteJob(job models.Job) *utils.GenericError
	QueueJobs(jobs []models.Job)
}

func NewJobService(
	context context.Context,
	logger hclog.Logger,
	jobRepo job.JobRepo,
	queue queue.JobQueueService,
	projectRepo project.ProjectRepo,
	dispatcher *utils.Dispatcher,
	asyncTaskService async_task.AsyncTaskService,
) JobService {
	service := &jobService{
		jobRepo:          jobRepo,
		projectRepo:      projectRepo,
		Queue:            queue,
		Ctx:              context,
		logger:           logger,
		dispatcher:       dispatcher,
		asyncTaskManager: asyncTaskService,
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
func (jobService *jobService) GetJob(job models.Job) (*models.Job, *utils.GenericError) {
	jobMangerGetOneError := jobService.jobRepo.GetOneByID(&job)
	if jobMangerGetOneError != nil {
		return nil, jobMangerGetOneError
	}

	return &job, nil
}

// BatchInsertJobs creates jobs in batches
func (jobService *jobService) BatchInsertJobs(requestId string, jobs []models.Job) ([]uint64, *utils.GenericError) {
	if len(jobs) < 1 {
		return nil, nil
	}

	for _, job := range jobs {
		if job.Spec != "" {
			if _, err := cron.Parse(job.Spec); err != nil {
				return nil, utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job spec is not valid %s", job.Spec))
			}
		} else {
			return nil, utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job spec is not valid %s", job.Spec))
		}

		if job.Timezone == "" || job.Timezone == "Local" {
			return nil, utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job timezone is not valid, provided timezone is %s", job.Timezone))
		}

		_, err := time.LoadLocation(job.Timezone)
		if err != nil {
			return nil, utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job timezone is not valid, provided timezone is %s", job.Timezone))
		}
	}

	var projectIds []uint64

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

	jobsBytes, marshalErr := json.Marshal(jobs)
	if marshalErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, fmt.Sprintf("failed to convert json to string"))
	}

	taskIds, addTaskErr := jobService.asyncTaskManager.AddTasks(string(jobsBytes), requestId, constants.CreateJobAsyncTaskService)
	if addTaskErr != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, fmt.Sprintf("failed to create async tasks %s", addTaskErr.Message))
	}

	jobService.dispatcher.NoBlockQueue(func(successChannel chan any, errorChannel chan any) {
		defer func() {
			close(successChannel)
			close(errorChannel)
		}()
		inProgressUpdateTaskErr := jobService.asyncTaskManager.UpdateTasksById(taskIds[0], models.AsyncTaskInProgress, "")
		if inProgressUpdateTaskErr != nil {
			jobService.logger.Error("failed to update an async task", inProgressUpdateTaskErr, "; new state:", models.AsyncTaskInProgress)
			return
		}

		insertedIds, iErr := jobService.jobRepo.BatchInsertJobs(jobs)
		if iErr != nil {
			errJson, errJsonErr := json.Marshal(utils.HTTPGenericError(http.StatusInternalServerError, fmt.Sprintf("failed to batch insert job repository: %v", iErr.Message)))
			if errJsonErr != nil {
				jobService.logger.Error("failed to save error out for an async task", errJsonErr)
				return
			}
			updateTaskErr := jobService.asyncTaskManager.UpdateTasksById(taskIds[0], models.AsyncTaskFail, string(errJson))
			if updateTaskErr != nil {
				jobService.logger.Error("failed to update an async task", updateTaskErr, "; new state:", models.AsyncTaskFail)
				return
			}
			jobService.logger.Error("failed to batch insert jobs", iErr)
			return
		}

		schedulerTime := scheduler0time.GetSchedulerTime()
		now := schedulerTime.GetTime(time.Now())

		for i, insertedId := range insertedIds {
			jobs[i].ID = insertedId
			jobs[i].DateCreated = now
			jobs[i].LastExecutionDate = now
		}

		jobService.QueueJobs(jobs)
		jobsJson, errJsonErr := json.Marshal(jobs)
		if errJsonErr != nil {
			jobService.logger.Error("failed to save error out for an async task", errJsonErr)
			return
		}
		updateTaskErr := jobService.asyncTaskManager.UpdateTasksById(taskIds[0], models.AsyncTaskSuccess, string(jobsJson))
		if updateTaskErr != nil {
			jobService.logger.Error("failed to update an async task", updateTaskErr, "; new state:", models.AsyncTaskSuccess)
			return
		}
	})

	return taskIds, nil
}

// UpdateJob updates job with ID in transformer. Note that cron expression of job cannot be updated.
func (jobService *jobService) UpdateJob(job models.Job) (*models.Job, *utils.GenericError) {
	currentJobState := models.Job{
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
func (jobService *jobService) DeleteJob(job models.Job) *utils.GenericError {
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

func (jobService *jobService) QueueJobs(jobs []models.Job) {
	jobService.Queue.Queue(jobs)
}
