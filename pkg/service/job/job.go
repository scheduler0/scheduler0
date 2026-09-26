package job

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"scheduler0-private/pkg/constants"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/repository/executor"
	"scheduler0-private/pkg/repository/job"
	"scheduler0-private/pkg/repository/project"
	"scheduler0-private/pkg/scheduler0time"
	"scheduler0-private/pkg/service/account"
	"scheduler0-private/pkg/service/async_task"
	"scheduler0-private/pkg/service/job_execution_service"
	"scheduler0-private/pkg/service/queue"
	"scheduler0-private/pkg/utils"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/robfig/cron"
)

type jobService struct {
	jobRepo                job.JobRepo
	jobExecutorRepo        executor.JobExecutorRepo
	projectRepo            project.ProjectRepo
	Queue                  queue.JobQueueService
	Ctx                    context.Context
	logger                 hclog.Logger
	dispatcher             *utils.Dispatcher
	asyncTaskManager       async_task.AsyncTaskService
	jobExecutionLogService job_execution_service.JobExecutionLogService
	accountService         account.AccountService
}

type JobService interface {
	GetJobsByProjectID(projectID uint64, offset uint64, limit uint64, orderByColumn string, orderByDirection string) (*models.PaginatedJob, *utils.GenericError)
	GetJobsByAccountID(accountID uint64, projectID *uint64, offset uint64, limit uint64, orderByColumn string, orderByDirection string) (*models.PaginatedJob, *utils.GenericError)
	GetJob(job models.Job) (*models.Job, *utils.GenericError)
	BatchInsertJobs(requestId string, jobs []models.Job) ([]uint64, *utils.GenericError)
	BatchInsertJobsSync(requestId string, jobs []models.Job) ([]models.Job, *utils.GenericError)
	UpdateJob(job models.Job) (*models.Job, *utils.GenericError)
	DeleteJob(job models.Job) *utils.GenericError
	DeleteJobsByProjectID(projectID uint64, accountId uint64, deletedBy string) *utils.GenericError
	QueueJobs(jobs []models.Job)
}

func NewJobService(
	context context.Context,
	logger hclog.Logger,
	jobRepo job.JobRepo,
	queue queue.JobQueueService,
	projectRepo project.ProjectRepo,
	jobExecutorRepo executor.JobExecutorRepo,
	dispatcher *utils.Dispatcher,
	asyncTaskService async_task.AsyncTaskService,
	jobExecutionLogService job_execution_service.JobExecutionLogService,
	accountService account.AccountService,
) JobService {
	service := &jobService{
		jobRepo:          jobRepo,
		projectRepo:      projectRepo,
		jobExecutorRepo:  jobExecutorRepo,
		Queue:            queue,
		Ctx:              context,
		logger:           logger,
		dispatcher:       dispatcher,
		asyncTaskManager: asyncTaskService,
		accountService:   accountService,
	}

	return service
}

// normalizeJobDatesToTimezone ensures wall-clock times are interpreted in the job's timezone
// If the parsed time has a UTC location (common when no offset is provided), and the job
// timezone is non-UTC, it re-constructs the time in the job's location without shifting
// the wall time components.
func normalizeJobDatesToTimezone(job *models.Job) {
	if job == nil || job.Timezone == "" {
		return
	}
	loc, err := time.LoadLocation(job.Timezone)
	if err != nil {
		return
	}

	if !job.StartDate.IsZero() {
		if job.Timezone != "UTC" && job.StartDate.Location() == time.UTC {
			sd := job.StartDate
			job.StartDate = time.Date(sd.Year(), sd.Month(), sd.Day(), sd.Hour(), sd.Minute(), sd.Second(), sd.Nanosecond(), loc)
		}
	}

	if !job.EndDate.IsZero() {
		if job.Timezone != "UTC" && job.EndDate.Location() == time.UTC {
			ed := job.EndDate
			job.EndDate = time.Date(ed.Year(), ed.Month(), ed.Day(), ed.Hour(), ed.Minute(), ed.Second(), ed.Nanosecond(), loc)
		}
	}
}

// GetJobsByProjectID returns a paginated set of jobs for a project
func (jobService *jobService) GetJobsByProjectID(projectID uint64, offset uint64, limit uint64, orderByColumn string, orderByDirection string) (*models.PaginatedJob, *utils.GenericError) {
	startTime := time.Now()
	jobService.logger.Info("getting jobs by project ID", "projectId", projectID, "offset", offset, "limit", limit, "orderBy", orderByColumn, "orderDirection", orderByDirection)

	if limit > constants.MaxListLimit {
		jobService.logger.Warn("limit exceeds maximum", "limit", limit, "maxLimit", constants.MaxListLimit)
		return nil, utils.HTTPGenericError(http.StatusTooManyRequests, fmt.Sprintf("too many jobs. limit should be less than %d", constants.MaxListLimit))
	}

	if limit < 1 {
		jobService.logger.Warn("invalid limit", "limit", limit)
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "limit should be greater than 0")
	}

	if offset < 0 {
		jobService.logger.Warn("invalid offset", "offset", offset)
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "offset should be greater than 0")
	}

	jobService.logger.Debug("getting total job count for project", "projectId", projectID)
	count, getCountError := jobService.jobRepo.GetJobsTotalCountByProjectID(projectID)
	if getCountError != nil {
		jobService.logger.Error("failed to get total job count", "error", getCountError, "projectId", projectID)
		return nil, getCountError
	}
	jobService.logger.Debug("total job count retrieved", "projectId", projectID, "totalCount", count)

	if uint64(count) < offset {
		jobService.logger.Debug("offset exceeds total count, adjusting", "originalOffset", offset, "totalCount", count)
		offset = uint64(count)
	}

	jobService.logger.Debug("fetching jobs from repository", "projectId", projectID, "offset", offset, "limit", limit)
	jobManagers, err := jobService.jobRepo.GetAllByProjectID(projectID, offset, limit, orderByColumn, orderByDirection)
	if err != nil {
		jobService.logger.Error("failed to get jobs from repository", "error", err, "projectId", projectID)
		return nil, err
	}
	jobService.logger.Debug("jobs retrieved from repository", "projectId", projectID, "jobCount", len(jobManagers))

	paginatedJobs := models.PaginatedJob{}
	paginatedJobs.Data = jobManagers
	paginatedJobs.Limit = limit
	paginatedJobs.Total = count
	paginatedJobs.Offset = offset

	duration := time.Since(startTime)
	jobService.logger.Info("get jobs by project ID completed", "projectId", projectID, "returnedCount", len(jobManagers), "total", count, "offset", offset, "limit", limit, "duration", duration, "durationMs", duration.Milliseconds())
	return &paginatedJobs, nil
}

// GetJobsByAccountID returns a paginated set of jobs for an account, optionally filtered by project
func (jobService *jobService) GetJobsByAccountID(accountID uint64, projectID *uint64, offset uint64, limit uint64, orderByColumn string, orderByDirection string) (*models.PaginatedJob, *utils.GenericError) {
	startTime := time.Now()
	pid := uint64(0)
	if projectID != nil {
		pid = *projectID
	}
	jobService.logger.Info("getting jobs by account ID", "accountId", accountID, "projectId", pid, "offset", offset, "limit", limit, "orderBy", orderByColumn, "orderDirection", orderByDirection)

	if limit > constants.MaxListLimit {
		jobService.logger.Warn("limit exceeds maximum", "limit", limit, "maxLimit", constants.MaxListLimit)
		return nil, utils.HTTPGenericError(http.StatusTooManyRequests, fmt.Sprintf("too many jobs. limit should be less than %d", constants.MaxListLimit))
	}

	if limit < 1 {
		jobService.logger.Warn("invalid limit", "limit", limit)
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "limit should be greater than 0")
	}

	if offset < 0 {
		jobService.logger.Warn("invalid offset", "offset", offset)
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "offset should be greater than 0")
	}

	jobService.logger.Debug("fetching paginated jobs from repository", "accountId", accountID, "projectId", pid, "offset", offset, "limit", limit)
	jobManagers, total, err := jobService.jobRepo.GetJobsPaginated(accountID, pid, offset, limit, orderByColumn, orderByDirection)
	if err != nil {
		jobService.logger.Error("failed to get paginated jobs from repository", "error", err, "accountId", accountID, "projectId", pid)
		return nil, err
	}
	jobService.logger.Debug("paginated jobs retrieved from repository", "accountId", accountID, "jobCount", len(jobManagers), "total", total)

	paginatedJobs := models.PaginatedJob{}
	paginatedJobs.Data = jobManagers
	paginatedJobs.Limit = limit
	paginatedJobs.Total = total
	paginatedJobs.Offset = offset

	duration := time.Since(startTime)
	jobService.logger.Info("get jobs by account ID completed", "accountId", accountID, "projectId", pid, "returnedCount", len(jobManagers), "total", total, "offset", offset, "limit", limit, "duration", duration, "durationMs", duration.Milliseconds())
	return &paginatedJobs, nil
}

// GetJob returns a job with ID that matched ID of transformer.
// job.AccountId must be set by the caller; the lookup is account-scoped.
func (jobService *jobService) GetJob(job models.Job) (*models.Job, *utils.GenericError) {
	startTime := time.Now()
	jobService.logger.Info("getting job", "jobId", job.ID, "accountId", job.AccountId)

	if job.AccountId == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	jobMangerGetOneError := jobService.jobRepo.GetOneByID(&job)
	if jobMangerGetOneError != nil {
		jobService.logger.Error("failed to get job from repository", "error", jobMangerGetOneError, "jobId", job.ID, "accountId", job.AccountId)
		return nil, jobMangerGetOneError
	}

	duration := time.Since(startTime)
	jobService.logger.Info("get job completed", "jobId", job.ID, "projectId", job.ProjectID, "accountId", job.AccountId, "duration", duration, "durationMs", duration.Milliseconds())
	return &job, nil
}

// validateJob validates a single job and returns an error if validation fails
// isUpdate should be true when validating a job update (which allows past dates for already-started jobs)
func (jobService *jobService) validateJob(job *models.Job, hasJobPayloadOf1Mb bool, hasJobRetryMaxBy5 bool, isUpdate bool) *utils.GenericError {
	// Validate cron spec if provided
	if job.Spec != "" {
		if _, err := cron.Parse(job.Spec); err != nil {
			return utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job spec is not valid %s", job.Spec))
		}
	} else {
		// If no spec, startDate is required for one-time jobs
		if job.StartDate.IsZero() {
			return utils.HTTPGenericError(http.StatusBadRequest, "either spec or startDate is required. For one-time jobs, provide only startDate")
		}
	}

	if job.Timezone == "" || job.Timezone == "Local" {
		return utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job timezone is not valid, provided timezone is %s", job.Timezone))
	}

	_, err := time.LoadLocation(job.Timezone)
	if err != nil {
		return utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job timezone is not valid, provided timezone is %s", job.Timezone))
	}

	if job.ExecutorId == nil {
		return utils.HTTPGenericError(http.StatusBadRequest, "job executor is required")
	}

	if len([]byte(job.Data)) > constants.DefaultJobPayloadMaxBytes && !hasJobPayloadOf1Mb {
		return utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job data exceeds maximum size of 3KB. Current size: %d bytes", len(job.Data)))
	}

	if len([]byte(job.Data)) > constants.IncreasedJobPayloadMaxBytes && hasJobPayloadOf1Mb {
		return utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job data exceeds maximum size of 1MB. Current size: %d bytes", len(job.Data)))
	}

	// Compare dates using the job's timezone
	loc, _ := time.LoadLocation(job.Timezone) // timezone validity is already validated above
	nowInLoc := time.Now().In(loc)

	// For new jobs, reject past dates. For updates, allow past dates (job may already be running).
	if !isUpdate {
		if !job.StartDate.IsZero() {
			startInLoc := job.StartDate.In(loc)
			if startInLoc.Before(nowInLoc) {
				jobService.logger.Debug("job start date is in the past", "job", startInLoc, "now", nowInLoc)
				return utils.HTTPGenericError(http.StatusBadRequest, "job start date is in the past")
			}
		}

		if !job.EndDate.IsZero() {
			endInLoc := job.EndDate.In(loc)
			if endInLoc.Before(nowInLoc) {
				jobService.logger.Debug("job end date is in the past", "job", endInLoc, "now", nowInLoc)
				return utils.HTTPGenericError(http.StatusBadRequest, "job end date is in the past")
			}
		}
	}

	if !job.StartDate.IsZero() && !job.EndDate.IsZero() {
		startInLoc := job.StartDate.In(loc)
		endInLoc := job.EndDate.In(loc)
		if endInLoc.Before(startInLoc) {
			jobService.logger.Debug("job end date is before start date", "job", endInLoc, "start date", startInLoc)
			return utils.HTTPGenericError(http.StatusBadRequest, "job end date is before start date")
		}
	}

	if job.RetryMax > constants.DefaultJobRetryMax && !hasJobRetryMaxBy5 {
		return utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job retry max is greater than %d", constants.DefaultJobRetryMax))
	}

	if job.RetryMax > constants.IncreasedJobRetryMax && hasJobRetryMaxBy5 {
		return utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("job retry max is greater than %d", constants.IncreasedJobRetryMax))
	}

	// Validate status
	if job.Status == "" {
		job.Status = models.JobStatusActive
	} else if job.Status != models.JobStatusActive && job.Status != models.JobStatusInactive {
		return utils.HTTPGenericError(http.StatusBadRequest, "job status must be 'active' or 'inactive'")
	}

	return nil
}

// validateBatch runs the shared pre-insert checks used by both the async BatchInsertJobs
// and the synchronous BatchInsertJobsSync: job-count bounds, per-account feature-aware
// validation (with date normalization), and project/executor existence + ownership. It
// mutates jobs in place (date normalization) and returns nil when the batch is valid.
func (jobService *jobService) validateBatch(jobs []models.Job) *utils.GenericError {
	if len(jobs) < 1 || len(jobs) > 100 {
		jobService.logger.Warn("invalid job count", "jobCount", len(jobs), "min", 1, "max", 100)
		return utils.HTTPGenericError(http.StatusBadRequest, fmt.Sprintf("number of jobs should be between 1 and 100. Current number of jobs: %d", len(jobs)))
	}

	accountId := jobs[0].AccountId
	jobService.logger.Debug("getting account features", "accountId", accountId)

	if accountId != 1 {
		features, getFeaturesErr := jobService.accountService.GetFeatures(accountId)
		if getFeaturesErr != nil {
			jobService.logger.Error("failed to get account features", "error", getFeaturesErr, "accountId", accountId)
			return getFeaturesErr
		}
		jobService.logger.Debug("account features retrieved", "accountId", accountId, "featureCount", len(*features))

		hasJobPayloadOf1Mb := false
		hasJobRetryMaxBy5 := false

		for _, feature := range *features {
			if feature.Feature == constants.IncreasedJobPayloadSizeTo1MBFeature {
				hasJobPayloadOf1Mb = true
			}
			if feature.Feature == constants.IncreasedRetryMaxByFiveFeature {
				hasJobRetryMaxBy5 = true
			}
		}

		jobService.logger.Debug("hasJobPayloadOf1Mb", "hasJobPayloadOf1Mb", hasJobPayloadOf1Mb)
		jobService.logger.Debug("hasJobRetryMaxOf5", "hasJobRetryMaxBy5", hasJobRetryMaxBy5)
		jobService.logger.Debug("validating jobs", "jobCount", len(jobs))
		for i := range jobs {
			normalizeJobDatesToTimezone(&jobs[i])
			if err := jobService.validateJob(&jobs[i], hasJobPayloadOf1Mb, hasJobRetryMaxBy5, false); err != nil {
				jobService.logger.Error("job validation failed", "error", err, "jobIndex", i, "jobId", jobs[i].ID)
				return err
			}
		}
		jobService.logger.Debug("all jobs validated successfully", "jobCount", len(jobs))
	}

	// Use sets to collect unique project and executor IDs
	projectIdsSet := make(map[uint64]bool)
	executorIdsSet := make(map[uint64]bool)

	for _, job := range jobs {
		projectIdsSet[job.ProjectID] = true
		executorIdsSet[*job.ExecutorId] = true
	}

	// Convert sets to slices for repository calls
	var projectIds []uint64
	var executorIds []uint64

	for projectId := range projectIdsSet {
		projectIds = append(projectIds, projectId)
	}

	for executorId := range executorIdsSet {
		executorIds = append(executorIds, executorId)
	}

	jobService.logger.Debug("collecting unique project and executor IDs", "projectIds", projectIds, "executorIds", executorIds, "uniqueProjectCount", len(projectIds), "uniqueExecutorCount", len(executorIds))

	jobService.logger.Debug("fetching projects from repository", "projectIds", projectIds)
	projects, err := jobService.projectRepo.GetBatchProjectsByIDs(projectIds)
	if err != nil {
		jobService.logger.Error("failed to get projects from repository", "error", err, "projectIds", projectIds)
		return err
	}
	jobService.logger.Debug("projects retrieved from repository", "requestedCount", len(projectIds), "returnedCount", len(projects))

	if len(projects) != len(projectIds) {
		jobService.logger.Warn("some projects not found", "requestedCount", len(projectIds), "foundCount", len(projects), "projectIds", projectIds)
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("a project in the payload does not exist. project id %v", projectIds))
	}

	jobService.logger.Debug("fetching executors from repository", "executorIds", executorIds)
	executors, err := jobService.jobExecutorRepo.BatchGetByIds(executorIds)
	if err != nil {
		jobService.logger.Error("failed to get executors from repository", "error", err, "executorIds", executorIds)
		return err
	}
	jobService.logger.Debug("executors retrieved from repository", "requestedCount", len(executorIds), "returnedCount", len(executors))

	if len(executors) != len(executorIds) {
		jobService.logger.Warn("some executors not found", "requestedCount", len(executorIds), "foundCount", len(executors), "executorIds", executorIds)
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("a executor in the payload does not exist. executor id %v", executorIds))
	}

	projectsByID := make(map[uint64]models.Project, len(projects))
	for _, project := range projects {
		projectsByID[project.ID] = project
	}
	executorsByID := make(map[uint64]models.JobExecutor, len(executors))
	for _, executor := range executors {
		executorsByID[executor.ID] = executor
	}

	for _, job := range jobs {
		project, found := projectsByID[job.ProjectID]
		if !found {
			return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("a project in the payload does not exist. project id %v", job.ProjectID))
		}
		if project.AccountId != job.AccountId {
			jobService.logger.Warn("project does not belong to account", "projectId", job.ProjectID, "projectAccountId", project.AccountId, "jobAccountId", job.AccountId)
			return utils.HTTPGenericError(http.StatusForbidden, "project does not belong to account")
		}

		executor, found := executorsByID[*job.ExecutorId]
		if !found {
			return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("a executor in the payload does not exitst. executor id %v", *job.ExecutorId))
		}
		if executor.AccountId != job.AccountId {
			jobService.logger.Warn("executor does not belong to account", "executorId", *job.ExecutorId, "executorAccountId", executor.AccountId, "jobAccountId", job.AccountId)
			return utils.HTTPGenericError(http.StatusForbidden, "executor does not belong to account")
		}
	}

	return nil
}

func (jobService *jobService) BatchInsertJobs(requestId string, jobs []models.Job) ([]uint64, *utils.GenericError) {
	startTime := time.Now()
	jobService.logger.Info("batch inserting jobs", "requestId", requestId, "jobCount", len(jobs))

	if vErr := jobService.validateBatch(jobs); vErr != nil {
		return nil, vErr
	}

	jobService.logger.Debug("marshaling jobs to JSON", "jobCount", len(jobs))
	jobsBytes, marshalErr := json.Marshal(jobs)
	if marshalErr != nil {
		jobService.logger.Error("failed to marshal jobs to JSON", "error", marshalErr, "jobCount", len(jobs))
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, fmt.Sprintf("failed to convert json to string"))
	}
	jobService.logger.Debug("jobs marshaled to JSON", "jobCount", len(jobs), "jsonSize", len(jobsBytes))

	jobService.logger.Debug("creating async tasks", "requestId", requestId, "service", constants.CreateJobAsyncTaskService)
	taskIds, addTaskErr := jobService.asyncTaskManager.AddTasks(string(jobsBytes), requestId, constants.CreateJobAsyncTaskService, jobs[0].AccountId)
	if addTaskErr != nil {
		jobService.logger.Error("failed to create async tasks", "error", addTaskErr, "requestId", requestId)
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, fmt.Sprintf("failed to create async tasks %s", addTaskErr.Message))
	}
	jobService.logger.Debug("async tasks created", "requestId", requestId, "taskIds", taskIds)

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

		jobService.logger.Info("batch inserting jobs in async task", "requestId", requestId, "jobCount", len(jobs))

		insertedIds, iErr := jobService.jobRepo.BatchInsertJobs(jobs)
		if iErr != nil {
			jobService.logger.Error("batch insert jobs failed in async task", "error", iErr, "requestId", requestId, "jobCount", len(jobs))
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

		// Filter out inactive jobs before queueing
		activeJobs := make([]models.Job, 0, len(jobs))
		for _, job := range jobs {
			if job.Status == models.JobStatusActive {
				activeJobs = append(activeJobs, job)
			}
		}

		jobService.logger.Debug("queueing inserted jobs", "requestId", requestId, "insertedCount", len(insertedIds), "activeCount", len(activeJobs))
		jobService.QueueJobs(activeJobs)

		jobService.logger.Debug("marshaling jobs for async task success", "requestId", requestId, "jobCount", len(jobs))
		jobsJson, errJsonErr := json.Marshal(jobs)
		if errJsonErr != nil {
			jobService.logger.Error("failed to marshal jobs for async task success", "error", errJsonErr, "requestId", requestId)
			return
		}

		jobService.logger.Debug("updating async task to success", "requestId", requestId, "taskId", taskIds[0])
		updateTaskErr := jobService.asyncTaskManager.UpdateTasksById(taskIds[0], models.AsyncTaskSuccess, string(jobsJson))
		if updateTaskErr != nil {
			jobService.logger.Error("failed to update async task to success", "error", updateTaskErr, "requestId", requestId, "taskId", taskIds[0])
			return
		}

		duration := time.Since(startTime)
		jobService.logger.Info("batch insert jobs async task completed successfully", "requestId", requestId, "insertedCount", len(insertedIds), "duration", duration, "durationMs", duration.Milliseconds())
	})

	duration := time.Since(startTime)
	jobService.logger.Info("batch insert jobs initiated", "requestId", requestId, "jobCount", len(jobs), "taskIds", taskIds, "duration", duration, "durationMs", duration.Milliseconds())
	return taskIds, nil
}

// BatchInsertJobsSync validates and inserts the jobs inline (no async task) and queues them,
// returning the created jobs with their IDs, DateCreated and LastExecutionDate populated. It
// is used by the /api/v1/ai/schedule endpoint, which needs the created jobs in the response.
func (jobService *jobService) BatchInsertJobsSync(requestId string, jobs []models.Job) ([]models.Job, *utils.GenericError) {
	startTime := time.Now()
	jobService.logger.Info("batch inserting jobs (sync)", "requestId", requestId, "jobCount", len(jobs))

	if vErr := jobService.validateBatch(jobs); vErr != nil {
		return nil, vErr
	}

	insertedIds, iErr := jobService.jobRepo.BatchInsertJobs(jobs)
	if iErr != nil {
		jobService.logger.Error("batch insert jobs (sync) failed", "error", iErr, "requestId", requestId, "jobCount", len(jobs))
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, fmt.Sprintf("failed to batch insert job repository: %v", iErr.Message))
	}

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())
	for i, insertedId := range insertedIds {
		jobs[i].ID = insertedId
		jobs[i].DateCreated = now
		jobs[i].LastExecutionDate = now
	}

	// Filter out inactive jobs before queueing
	activeJobs := make([]models.Job, 0, len(jobs))
	for _, job := range jobs {
		if job.Status == models.JobStatusActive {
			activeJobs = append(activeJobs, job)
		}
	}

	jobService.logger.Debug("queueing inserted jobs (sync)", "requestId", requestId, "insertedCount", len(insertedIds), "activeCount", len(activeJobs))
	jobService.QueueJobs(activeJobs)

	duration := time.Since(startTime)
	jobService.logger.Info("batch insert jobs (sync) completed", "requestId", requestId, "insertedCount", len(insertedIds), "durationMs", duration.Milliseconds())
	return jobs, nil
}

// UpdateJob updates job with ID in transformer. Note that cron expression of job cannot be updated.
func (jobService *jobService) UpdateJob(job models.Job) (*models.Job, *utils.GenericError) {
	startTime := time.Now()
	jobService.logger.Info("updating job", "jobId", job.ID, "projectId", job.ProjectID, "accountId", job.AccountId)

	if job.AccountId == 0 {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	// Fetch existing job (account-scoped) and merge any fields that were not supplied in the request.
	existing := models.Job{ID: job.ID, AccountId: job.AccountId}
	if getErr := jobService.jobRepo.GetOneByID(&existing); getErr != nil {
		jobService.logger.Error("failed to get existing job for merge", "error", getErr, "jobId", job.ID, "accountId", job.AccountId)
		return nil, getErr
	}
	if existing.AccountId != job.AccountId {
		jobService.logger.Warn("job does not belong to account", "jobId", job.ID, "jobAccountId", existing.AccountId, "callerAccountId", job.AccountId)
		return nil, utils.HTTPGenericError(http.StatusNotFound, "job cannot be found")
	}
	if job.Timezone == "" {
		job.Timezone = existing.Timezone
		job.TimezoneOffset = existing.TimezoneOffset
	}
	if job.ExecutorId == nil {
		job.ExecutorId = existing.ExecutorId
	}
	if job.CreatedBy == "" {
		job.CreatedBy = existing.CreatedBy
	}
	if job.Spec == "" {
		job.Spec = existing.Spec
	}
	if job.StartDate.IsZero() {
		job.StartDate = existing.StartDate
	}
	if job.EndDate.IsZero() {
		job.EndDate = existing.EndDate
	}
	if job.Data == "" {
		job.Data = existing.Data
	}
	if job.Status == "" {
		job.Status = existing.Status
	}
	if job.RetryMax == 0 {
		job.RetryMax = existing.RetryMax
	}

	accountId := job.AccountId
	hasJobPayloadOf1Mb := false
	hasJobRetryMaxBy5 := false

	if accountId != 1 {
		jobService.logger.Debug("getting account features", "accountId", accountId)
		features, getFeaturesErr := jobService.accountService.GetFeatures(accountId)
		if getFeaturesErr != nil {
			jobService.logger.Error("failed to get account features", "error", getFeaturesErr, "accountId", accountId)
			return nil, getFeaturesErr
		}
		jobService.logger.Debug("account features retrieved", "accountId", accountId, "featureCount", len(*features))

		for _, feature := range *features {
			if feature.Feature == constants.IncreasedJobPayloadSizeTo1MBFeature {
				hasJobPayloadOf1Mb = true
			}
			if feature.Feature == constants.IncreasedRetryMaxByFiveFeature {
				hasJobRetryMaxBy5 = true
			}
		}

		jobService.logger.Debug("hasJobPayloadOf1Mb", "hasJobPayloadOf1Mb", hasJobPayloadOf1Mb)
		jobService.logger.Debug("hasJobRetryMaxOf5", "hasJobRetryMaxBy5", hasJobRetryMaxBy5)
	}
	// Ensure the target project belongs to the same account
	jobService.logger.Debug("verifying project ownership", "projectId", job.ProjectID, "accountId", job.AccountId)
	projectModel := models.Project{ID: job.ProjectID, AccountId: job.AccountId}
	if getProjectErr := jobService.projectRepo.GetOneByID(&projectModel); getProjectErr != nil {
		jobService.logger.Error("failed to get project", "error", getProjectErr, "projectId", job.ProjectID)
		return nil, getProjectErr
	}
	if projectModel.AccountId != job.AccountId {
		jobService.logger.Warn("project does not belong to account", "projectId", job.ProjectID, "projectAccountId", projectModel.AccountId, "jobAccountId", job.AccountId)
		return nil, utils.HTTPGenericError(http.StatusForbidden, "project does not belong to account")
	}
	jobService.logger.Debug("project ownership verified", "projectId", job.ProjectID)

	// Ensure the target executor belongs to the same account
	if job.ExecutorId != nil {
		jobService.logger.Debug("verifying executor ownership", "executorId", *job.ExecutorId, "accountId", job.AccountId)
		executorModel, getExecErr := jobService.jobExecutorRepo.GetOneByID(*job.ExecutorId, job.AccountId)
		if getExecErr != nil {
			jobService.logger.Error("failed to get executor", "error", getExecErr, "executorId", *job.ExecutorId)
			return nil, getExecErr
		}
		if executorModel.AccountId != job.AccountId {
			jobService.logger.Warn("executor does not belong to account", "executorId", *job.ExecutorId, "executorAccountId", executorModel.AccountId, "jobAccountId", job.AccountId)
			return nil, utils.HTTPGenericError(http.StatusForbidden, "executor does not belong to account")
		}
		jobService.logger.Debug("executor ownership verified", "executorId", *job.ExecutorId)
	}

	jobService.logger.Debug("validating job", "jobId", job.ID)
	normalizeJobDatesToTimezone(&job)
	if accountId != 1 {
		if err := jobService.validateJob(&job, hasJobPayloadOf1Mb, hasJobRetryMaxBy5, true); err != nil {
			jobService.logger.Error("job validation failed", "error", err, "jobId", job.ID)
			return nil, err
		}
		jobService.logger.Debug("job validated successfully", "jobId", job.ID)
	}

	jobService.logger.Debug("updating job in repository", "jobId", job.ID, "accountId", job.AccountId)
	rowsAffected, jobMangerUpdateOneError := jobService.jobRepo.UpdateOneByID(job)
	if jobMangerUpdateOneError != nil {
		jobService.logger.Error("failed to update job in repository", "error", jobMangerUpdateOneError, "jobId", job.ID)
		return nil, jobMangerUpdateOneError
	}
	if rowsAffected < 1 {
		jobService.logger.Warn("no rows affected during job update", "jobId", job.ID, "accountId", job.AccountId)
		return nil, utils.HTTPGenericError(http.StatusNotFound, "job cannot be found")
	}
	jobService.logger.Debug("job updated in repository", "jobId", job.ID)

	jobService.logger.Debug("fetching updated job from repository", "jobId", job.ID, "accountId", job.AccountId)
	getErr := jobService.jobRepo.GetOneByID(&job)
	if getErr != nil {
		jobService.logger.Error("failed to get updated job from repository", "error", getErr, "jobId", job.ID)
		return nil, getErr
	}

	duration := time.Since(startTime)
	jobService.logger.Info("update job completed", "jobId", job.ID, "projectId", job.ProjectID, "accountId", job.AccountId, "duration", duration, "durationMs", duration.Milliseconds())
	return &job, nil
}

// DeleteJob deletes a job with ID in transformer
func (jobService *jobService) DeleteJob(job models.Job) *utils.GenericError {
	startTime := time.Now()
	jobService.logger.Info("deleting job", "jobId", job.ID, "accountId", job.AccountId)

	if job.AccountId == 0 {
		return utils.HTTPGenericError(http.StatusBadRequest, "account id is required")
	}

	jobService.logger.Debug("verifying job exists", "jobId", job.ID, "accountId", job.AccountId)
	jobModel := models.Job{ID: job.ID, AccountId: job.AccountId}
	err := jobService.jobRepo.GetOneByID(&jobModel)
	if err != nil {
		jobService.logger.Error("job not found for deletion", "error", err, "jobId", job.ID, "accountId", job.AccountId)
		return err
	}
	jobService.logger.Debug("job found, proceeding with deletion", "jobId", job.ID)

	count, delError := jobService.jobRepo.DeleteOneByID(job)
	if delError != nil {
		jobService.logger.Error("failed to delete job from repository", "error", delError, "jobId", job.ID)
		return utils.HTTPGenericError(http.StatusInternalServerError, delError.Message)
	}

	if count < 1 {
		jobService.logger.Warn("no rows affected during job deletion", "jobId", job.ID, "accountId", job.AccountId)
		return utils.HTTPGenericError(http.StatusNotFound, "job cannot be found")
	}

	duration := time.Since(startTime)
	jobService.logger.Info("delete job completed", "jobId", job.ID, "rowsAffected", count, "duration", duration, "durationMs", duration.Milliseconds())
	return nil
}

// DeleteJobsByProjectID deletes all jobs for a project owned by accountId
func (jobService *jobService) DeleteJobsByProjectID(projectID uint64, accountId uint64, deletedBy string) *utils.GenericError {
	startTime := time.Now()
	jobService.logger.Info("deleting jobs by project ID", "projectId", projectID, "accountId", accountId, "deletedBy", deletedBy)

	count, err := jobService.jobRepo.DeleteJobsByProjectID(projectID, accountId, deletedBy)
	if err != nil {
		jobService.logger.Error("failed to delete jobs from repository", "error", err, "projectId", projectID, "accountId", accountId)
		return err
	}

	duration := time.Since(startTime)
	jobService.logger.Info("delete jobs by project ID completed", "projectId", projectID, "accountId", accountId, "rowsAffected", count, "deletedBy", deletedBy, "duration", duration, "durationMs", duration.Milliseconds())
	return nil
}

func (jobService *jobService) QueueJobs(jobs []models.Job) {
	startTime := time.Now()
	jobService.logger.Info("queueing jobs for execution", "jobCount", len(jobs))

	for i, job := range jobs {
		jobService.logger.Debug("queueing job", "jobIndex", i+1, "totalJobs", len(jobs), "jobId", job.ID, "projectId", job.ProjectID, "accountId", job.AccountId)
	}

	jobService.Queue.Queue(jobs)

	duration := time.Since(startTime)
	jobService.logger.Info("queue jobs completed", "jobCount", len(jobs), "duration", duration, "durationMs", duration.Milliseconds())
}
