package job_recovery

import (
	"fmt"
	"github.com/robfig/cron"
	"log"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/job_executor"
	models "scheduler0/models"
	"scheduler0/repository"
	"sync"
	"time"
)

type JobRecovery struct {
	jobExecutor   *job_executor.JobExecutor
	jobRepo       repository.Job
	logger        *log.Logger
	recoveredJobs []models.JobModel
	mtx           sync.Mutex
}

func NewJobRecovery(logger *log.Logger, jobRepo repository.Job, jobExecutor *job_executor.JobExecutor) *JobRecovery {
	return &JobRecovery{
		jobExecutor: jobExecutor,
		logger:      logger,
		jobRepo:     jobRepo,
	}
}

func (jobRecovery *JobRecovery) Run() {
	jobRecovery.mtx.Lock()
	defer jobRecovery.mtx.Unlock()

	jobRecovery.logger.Println("recovering jobs.")
	configs := config.GetScheduler0Configurations(jobRecovery.logger)
	jobsStates := jobRecovery.jobExecutor.GetJobLogsForServer(fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port))

	jobsIds := []int64{}

	for _, jobsState := range jobsStates {
		jobsIds = append(jobsIds, jobsState.Data[0].ID)
	}

	jobsFromDb, err := jobRecovery.jobRepo.BatchGetJobsByID(jobsIds)
	if err != nil {
		jobRecovery.logger.Fatalln("failed to retrieve jobs from db")
	}

	jobRecovery.logger.Println("recovered ", len(jobsFromDb), " jobs")

	for _, job := range jobsFromDb {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobRecovery.logger.Fatalln(fmt.Sprintf("failed to parse spec %v", parseErr.Error()))
		}
		jobState := jobsStates[int(job.ID)]

		now := time.Now().UTC()
		executionTime := schedule.Next(jobState.ExecutionTime)
		job.ExecutionId = jobState.Data[0].ExecutionId
		job.LastExecutionDate = jobState.Data[0].LastExecutionDate
		if now.Before(executionTime) {
			jobRecovery.logger.Println("quick recovered job", job.ID, job.ExecutionId)
			go func(j models.JobModel, e time.Time) {
				time.Sleep(e.Sub(now))
				jobRecovery.jobExecutor.LogJobExecutionStateOnLeader([]models.JobModel{j}, constants.CommandTypePrepareJobExecutions)
			}(job, executionTime)
		} else {
			jobRecovery.jobExecutor.Run([]models.JobModel{job})
		}
		jobRecovery.recoveredJobs = append(jobRecovery.recoveredJobs, job)
	}
}

func (jobRecovery *JobRecovery) HandlePrepare(jobState models.JobStateLog) {
	jobRecovery.mtx.Lock()
	defer jobRecovery.mtx.Unlock()
	for _, job := range jobState.Data {
		if jobRecovery.IsRecovered(job.ID) {
			jobProcess := jobRecovery.jobExecutor.AddNewProcess(job)
			jobRecovery.jobExecutor.ExecutePendingJobs([]*models.JobProcess{jobProcess})
			jobRecovery.RemoveJobRecovery(job.ID)
		}
	}
}

// IsRecovered Check if a job is in recovered job queues
func (jobRecovery *JobRecovery) IsRecovered(jobID int64) bool {
	for _, recoveredJob := range jobRecovery.recoveredJobs {
		if recoveredJob.ID == jobID {
			return true
		}
	}

	return false
}

// GetRecovery Returns recovery object
func (jobRecovery *JobRecovery) GetRecovery(jobID int64) *models.JobModel {
	for _, recoveredJob := range jobRecovery.recoveredJobs {
		if recoveredJob.ID == jobID {
			return &recoveredJob
		}
	}

	return nil
}

// RemoveJobRecovery Removes a recovery object
func (jobRecovery *JobRecovery) RemoveJobRecovery(jobID int64) {
	jobIndex := -1

	for index, recoveredJob := range jobRecovery.recoveredJobs {
		if recoveredJob.ID == jobID {
			jobIndex = index
			break
		}
	}

	jobRecovery.recoveredJobs = append(jobRecovery.recoveredJobs[:jobIndex], jobRecovery.recoveredJobs[jobIndex+1:]...)
}
