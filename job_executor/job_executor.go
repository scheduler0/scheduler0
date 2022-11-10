package job_executor

import (
	"fmt"
	"github.com/robfig/cron"
	"log"
	"scheduler0/executor"
	"scheduler0/models"
	"scheduler0/repository"
	"time"
)

type JobExecutor struct {
	PendingJobs chan *models.JobProcess
	jobRepo     repository.Job
	logger      *log.Logger
}

func NewJobExecutor(logger *log.Logger, jobRepository repository.Job) *JobExecutor {
	return &JobExecutor{
		PendingJobs: make(chan *models.JobProcess, 100),
		jobRepo:     jobRepository,
		logger:      logger,
	}
}

// AddPendingJobToChannel this will execute a http job
func (jobExecutor *JobExecutor) AddPendingJobToChannel(jobProcess models.JobProcess) func() {
	return func() {
		jobExecutor.PendingJobs <- &jobProcess
	}
}

// ExecutePendingJobs executes and http job
func (jobExecutor *JobExecutor) ExecutePendingJobs(pendingJobs []models.JobProcess) {
	jobIDs := make([]int64, 0)
	for _, pendingJob := range pendingJobs {
		jobIDs = append(jobIDs, pendingJob.Job.ID)
	}

	jobs, batchGetError := jobExecutor.jobRepo.BatchGetJobsByID(jobIDs)
	if batchGetError != nil {
		jobExecutor.logger.Println(fmt.Sprintf("Batch Query Error:: %s", batchGetError.Message))
		return
	}

	getPendingJob := func(jobID int64) *models.JobProcess {
		for _, pendingJob := range pendingJobs {
			if pendingJob.Job.ID == jobID {
				return &pendingJob
			}
		}
		return nil
	}

	jobExecutor.logger.Println(fmt.Sprintf("Batched Queried %v", len(jobs)))

	jobsToExecute := make([]*models.JobModel, 0)

	for _, job := range jobs {
		pendingJob := getPendingJob(job.ID)
		if pendingJob != nil {
			jobsToExecute = append(jobsToExecute, pendingJob.Job)
		}
	}

	onSuccess := func(pendingJobs []*models.JobModel) {
		for _, pendingJob := range pendingJobs {
			//if jobProcessor.IsRecovered(pendingJob.ID) {
			//	jobProcessor.RemoveJobRecovery(pendingJob.ID)
			//	jobProcessor.AddJobs([]models.JobModel{*pendingJob}, nil)
			//}

			jobExecutor.logger.Println(fmt.Sprintf("Executed job %v", pendingJob.ID))
		}
	}

	onFail := func(pj []*models.JobModel, err error) {
		jobExecutor.logger.Println("HTTP REQUEST ERROR:: ", err.Error())
	}

	executorService := executor.NewService(jobExecutor.logger, jobsToExecute, onSuccess, onFail)
	executorService.ExecuteHTTP()
}

// ListenToChannelsUpdates periodically checks channels for updates
func (jobExecutor *JobExecutor) ListenToChannelsUpdates() {
	pendingJobs := make([]models.JobProcess, 0)
	ticker := time.NewTicker(time.Millisecond * 100)

	for {
		select {
		case pendingJob := <-jobExecutor.PendingJobs:
			pendingJobs = append(pendingJobs, *pendingJob)
		case <-ticker.C:
			if len(pendingJobs) > 0 {
				jobExecutor.ExecutePendingJobs(pendingJobs[0:])
				jobExecutor.logger.Println(fmt.Sprintf("%v Pending Jobs To Execute", len(pendingJobs[0:])))
				pendingJobs = pendingJobs[len(pendingJobs):]
			}
		}
	}
}

func (jobExecutor *JobExecutor) Run(jobs []models.JobModel) {
	for _, job := range jobs {
		jobProcess := models.JobProcess{
			Job:  &job,
			Cron: cron.New(),
		}

		cronAddJobErr := jobProcess.Cron.AddFunc(job.Spec, jobExecutor.AddPendingJobToChannel(jobProcess))

		jobProcess.Cron.Start()

		if cronAddJobErr != nil {
			jobExecutor.logger.Println("Error Add Cron JOb", cronAddJobErr.Error())
			return
		}
	}
}
