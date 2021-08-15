package process

import (
	"fmt"
	"github.com/go-pg/pg"
	"github.com/robfig/cron"
	"net/http"
	"scheduler0/server/managers/execution"
	"scheduler0/server/managers/job"
	"scheduler0/server/managers/project"
	"scheduler0/server/service"
	"scheduler0/server/transformers"
	"scheduler0/utils"
	"strconv"
	"strings"
	"sync"
	"time"
)

// JobProcessor handles executions of jobs
type JobProcessor struct {
	Cron              *cron.Cron
	RecoveredJobs     []RecoveredJob
	PendingJobs       chan *PendingJob
	MaxMemory         int64
	MaxCPU            int64
	DBConnection      *pg.DB
	PendingJobUpdates chan *PendingJob
	PendingJobCreates chan *PendingJob
}

// RecoverJobExecutions find jobs that could've not been executed due to timeout
func (jobProcessor *JobProcessor) RecoverJobExecutions(jobTransformers []transformers.Job) {
	manager := execution.Manager{}
	for _, jobTransformer := range jobTransformers {
		if jobProcessor.IsRecovered(jobTransformer.UUID) {
			continue
		}

		count, err, executionManagers := manager.FindJobExecutionPlaceholderByUUID(jobProcessor.DBConnection, jobTransformer.UUID)
		if err != nil {
			utils.Error(fmt.Sprintf("Error occurred while fetching execution mangers for jobs to be recovered %s", err.Message))
			continue
		}

		if count < 1 {
			continue
		}

		executionManager := executionManagers[0]
		schedule, parseErr := cron.Parse(jobTransformer.Spec)

		if parseErr != nil {
			utils.Error(fmt.Sprintf("Failed to create schedule%s", parseErr.Error()))
			continue
		}
		now := time.Now().UTC()
		executionTime := schedule.Next(executionManager.TimeAdded).UTC()

		if now.Before(executionTime) {
			executionTransformer := transformers.Execution{}
			executionTransformer.FromManager(executionManager)
			recovery := RecoveredJob{
				Execution: &executionTransformer,
				Job:       &jobTransformer,
			}
			jobProcessor.RecoveredJobs = append(jobProcessor.RecoveredJobs, recovery)
		}
	}
}

// IsRecovered Check if a job is in recovered job queues
func (jobProcessor *JobProcessor) IsRecovered(jobUUID string) bool {
	for _, recoveredJob := range jobProcessor.RecoveredJobs {
		if recoveredJob.Job.UUID == jobUUID {
			return true
		}
	}

	return false
}

// GetRecovery Returns recovery object
func (jobProcessor *JobProcessor) GetRecovery(jobUUID string) *RecoveredJob {
	for _, recoveredJob := range jobProcessor.RecoveredJobs {
		if recoveredJob.Job.UUID == jobUUID {
			return &recoveredJob
		}
	}

	return nil
}

// RemoveJobRecovery Removes a recovery object
func (jobProcessor *JobProcessor) RemoveJobRecovery(jobUUID string) {
	jobIndex := -1

	for index, recoveredJob := range jobProcessor.RecoveredJobs {
		if recoveredJob.Job.UUID == jobUUID {
			jobIndex = index
			break
		}
	}

	jobProcessor.RecoveredJobs = append(jobProcessor.RecoveredJobs[:jobIndex], jobProcessor.RecoveredJobs[jobIndex+1:]...)
}

// ExecuteHTTPJobs executes and http job
func (jobProcessor *JobProcessor) ExecuteHTTPJobs(pendingJobs []PendingJob) {
	jobUUIDs := []string{}
	for _, pendingJob := range pendingJobs {
		jobUUIDs = append(jobUUIDs, pendingJob.Job.UUID)
	}

	jobManager := job.Manager{}
	jobs, batchGetError := jobManager.BatchGetJobs(jobProcessor.DBConnection, jobUUIDs)
	if batchGetError != nil {
		utils.Error(fmt.Sprintf("Batch Query Error:: %s", batchGetError.Message))
	}

	getPendingJob := func(jobUUID string) *PendingJob {
		for _, pendingJob := range pendingJobs {
			if pendingJob.Job.UUID == jobUUID {
				return &pendingJob
			}
		}
		return nil
	}

	utils.Info(fmt.Sprintf("Batched Queried %v", len(jobs)))

	for _, job := range jobs {
		pendingJob := getPendingJob(job.UUID)
		go jobProcessor.PerformHTTPRequest(pendingJob)
	}
}

// PerformHTTPRequest sends http request for a job
func (jobProcessor *JobProcessor) PerformHTTPRequest(jobExec *PendingJob) {
	utils.Info(fmt.Sprintf("Running Job Execution for Job ID = %s with execution = %s",
		jobExec.Job.UUID, jobExec.Execution.UUID))

	var statusCode int
	startSecs := time.Now()

	r, err := http.Post(jobExec.Job.CallbackUrl, "application/json", strings.NewReader(jobExec.Job.Data))
	if err != nil {
		utils.Error("HTTP REQUEST ERROR", err.Error())
		statusCode = -1
	} else {
		statusCode = r.StatusCode
	}

	if r != nil {
		r.Body.Close()
	}

	utils.Info(fmt.Sprintf("Executed job %v", jobExec.Job.UUID))
	timeout := time.Now().Sub(startSecs).Nanoseconds()
	jobExec.Execution.TimeExecuted = time.Now().UTC()
	jobExec.Execution.ExecutionTime = timeout
	jobExec.Execution.StatusCode = strconv.Itoa(statusCode)

	jobProcessor.PendingJobUpdates <- jobExec

	if jobProcessor.IsRecovered(jobExec.Job.UUID) {
		jobProcessor.RemoveJobRecovery(jobExec.Job.UUID)
		jobProcessor.AddJobs([]transformers.Job{*jobExec.Job}, nil)
	} else {
		jobProcessor.PendingJobCreates <- jobExec
	}
}

// HTTPJobExecutor this will execute an http job
func (jobProcessor *JobProcessor) HTTPJobExecutor(jobTransformer *transformers.Job, executionManger *execution.Manager) func() {
	return func() {
		jobProcessor.PendingJobs <- &PendingJob{
			Job:       jobTransformer,
			Execution: executionManger,
		}
	}
}

// StartJobs the cron job process
func (jobProcessor *JobProcessor) StartJobs() {
	projectManager := project.ProjectManager{}

	totalProjectCount, err := projectManager.Count(jobProcessor.DBConnection)
	if err != nil {
		panic(err)
	}

	utils.Info("Total number of projects: ", totalProjectCount)

	projectService := service.ProjectService{
		DBConnection: jobProcessor.DBConnection,
	}

	projectTransformers, err := projectService.List(0, totalProjectCount)
	if err != nil {
		panic(err)
	}

	jobService := service.JobService{
		DBConnection: jobProcessor.DBConnection,
	}

	var wg sync.WaitGroup

	for _, projectTransformer := range projectTransformers.Data {
		wg.Add(1)

		jobManager := job.Manager{}

		jobsTotalCount, err := jobManager.GetJobsTotalCountByProjectUUID(jobProcessor.DBConnection, projectTransformer.UUID)
		if err != nil {
			panic(err)
		}

		utils.Info(fmt.Sprintf("Total number of jobs for project %v is %v : ", projectTransformer.ID, jobsTotalCount))
		paginatedJobTransformers, err := jobService.GetJobsByProjectUUID(
			projectTransformer.UUID, 0, jobsTotalCount, "date_created")

		jobTransformers := []transformers.Job{}

		for _, jobTransformer := range paginatedJobTransformers.Data {
			jobTransformers = append(jobTransformers, jobTransformer)
		}

		jobProcessor.RecoverJobExecutions(jobTransformers)

		utils.Info(fmt.Sprintf("Recovered %v Jobs for Project with ID: %v",
			len(jobProcessor.RecoveredJobs),
			projectTransformer.ID))

		go jobProcessor.AddJobs(paginatedJobTransformers.Data, &wg)
		wg.Wait()
	}

	jobProcessor.Cron.Start()

	go jobProcessor.ListenToChannelsUpdates()
}

// ListenToChannelsUpdates periodically checks channels for updates
func (jobProcessor *JobProcessor) ListenToChannelsUpdates() {
	pendingJobs := []PendingJob{}
	executionManager := execution.Manager{}
	executionManagerUpdates := []execution.Manager{}
	executionManagerCreates := []execution.Manager{}

	ticker := time.NewTicker(time.Millisecond * 100)

	for {
		select {
		case insertExecution := <-jobProcessor.PendingJobCreates:
			executionManagerCreates = append(executionManagerCreates, execution.Manager{
				JobID:       insertExecution.Job.ID,
				JobUUID:     insertExecution.Job.UUID,
				TimeAdded:   time.Now().UTC(),
				DateCreated: time.Now().UTC(),
			})
		case updateExecution := <-jobProcessor.PendingJobUpdates:
			executionManagerUpdates = append(executionManagerUpdates, *updateExecution.Execution)
		case pendingJob := <-jobProcessor.PendingJobs:
			pendingJobs = append(pendingJobs, *pendingJob)
		case <-ticker.C:
			if len(pendingJobs) > 0 {
				jobProcessor.ExecuteHTTPJobs(pendingJobs[0:len(pendingJobs)])
				utils.Info(fmt.Sprintf("%v Pending Jobs To Execute", len(pendingJobs[0:len(pendingJobs)])))
				pendingJobs = pendingJobs[len(pendingJobs):]
			}

			if len(executionManagerUpdates) > 0 {
				batchUpdateErr := executionManager.BatchUpdateExecutions(
					jobProcessor.DBConnection,
					executionManagerUpdates[0:],
				)
				if batchUpdateErr != nil {
					utils.Error(fmt.Sprintf("Batch Update Error:: %s", batchUpdateErr.Message))
				} else {
					utils.Green(fmt.Sprintf("Successfully Updated %v Executions",
						len(executionManagerUpdates[0:])))
				}
				executionManagerUpdates = executionManagerUpdates[len(executionManagerUpdates):]
			}

			if len(executionManagerCreates) > 0 {
				_, batchInsertErr := executionManager.
					BatchInsertExecutions(jobProcessor.DBConnection, executionManagerCreates[0:])
				if batchInsertErr != nil {
					utils.Error(fmt.Sprintf("Batch Insert Error:: %s", batchInsertErr.Message))
				} else {
					utils.Green(fmt.Sprintf("Successfully Inserted %v New Executions",
						len(executionManagerCreates[0:])))
				}
				executionManagerCreates = executionManagerCreates[len(executionManagerCreates):]
			}
		}
	}
}

// AddJobs adds a single job to the queue
func (jobProcessor *JobProcessor) AddJobs(jobTransformers []transformers.Job, wg *sync.WaitGroup) {
	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()
	executionManagers := []execution.Manager{}

	for _, jobTransformer := range jobTransformers {
		if recovery := jobProcessor.GetRecovery(jobTransformer.UUID); recovery != nil {
			go recovery.Run(jobProcessor)
			return
		}

		executionManager := execution.Manager{
			JobID:       jobTransformer.ID,
			JobUUID:     jobTransformer.UUID,
			TimeAdded:   time.Now().UTC(),
			DateCreated: time.Now().UTC(),
		}

		executionManagers = append(executionManagers, executionManager)
	}

	executionManager := execution.Manager{}

	uuids, createErr := executionManager.BatchInsertExecutions(jobProcessor.DBConnection, executionManagers)
	for i, _ := range executionManagers {
		executionManagers[i].UUID = uuids[i]
	}
	if createErr != nil {
		fmt.Println("Error Getting Execution", utils.Error(createErr.Message))
		return
	}

	for i, jobTransformer := range jobTransformers {
		cronAddJobErr := jobProcessor.Cron.AddFunc(jobTransformer.Spec, jobProcessor.HTTPJobExecutor(&jobTransformer, &executionManagers[i]))
		if cronAddJobErr != nil {
			fmt.Println("Error Add Cron JOb", cronAddJobErr.Error())
			return
		}
	}

	utils.Info(fmt.Sprintf("Queued %v Jobs", len(jobTransformers)))
}
