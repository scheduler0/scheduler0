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
	Cron          *cron.Cron
	RecoveredJobs []RecoveredJob
	PendingJobs   chan *PendingJob
	MaxMemory     int64
	MaxCPU        int64
	DBConnection  *pg.DB
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

// ExecuteHTTPJob executes and http job
func (jobProcessor *JobProcessor) ExecuteHTTPJob(jobTransformer *transformers.Job, executionManager *execution.Manager) {
	utils.Info(fmt.Sprintf("Running Job Execution for Job ID = %s with execution = %s",
		jobTransformer.UUID, executionManager.UUID))

	jobManager, transformError := jobTransformer.ToManager()
	if transformError != nil {
		utils.Error("Job Transform Error:", transformError.Error())
		return
	}
	getOneError := jobManager.GetOne(jobProcessor.DBConnection, jobManager.UUID)
	if getOneError != nil {
		utils.Error("Get One Job Error:", getOneError.Message)
		return
	}

	var statusCode int
	startSecs := time.Now()

	r, err := http.Post(jobTransformer.CallbackUrl, "application/json", strings.NewReader(jobTransformer.Data))
	if err != nil {
		statusCode = -1
	} else {
		statusCode = r.StatusCode
	}

	utils.Info(fmt.Sprintf("Executed job %v", jobTransformer.UUID))

	timeout := uint64(time.Now().Sub(startSecs).Milliseconds())

	executionManager.TimeExecuted = time.Now().UTC()
	executionManager.ExecutionTime = timeout
	executionManager.StatusCode = strconv.Itoa(statusCode)

	updatedRows, updateError := executionManager.UpdateOne(jobProcessor.DBConnection)
	if updateError != nil {
		utils.Error("Cannot Update Execution Error:", updateError.Message)
		return
	}

	if updatedRows < 1 {
		utils.Info("Failed to update execution without any error")
	}

	if jobProcessor.IsRecovered(jobTransformer.UUID) {
		jobProcessor.RemoveJobRecovery(jobTransformer.UUID)
		jobProcessor.AddJob(*jobTransformer, nil)
	} else {
		newExecutionManager := execution.Manager{
			JobID:       jobTransformer.ID,
			JobUUID:     jobTransformer.UUID,
			TimeAdded:   time.Now().UTC(),
			DateCreated: time.Now().UTC(),
		}
		_, createErr := newExecutionManager.CreateOne(jobProcessor.DBConnection)
		if createErr != nil {
			utils.Error("Error Creating New Placeholder Execution :", createErr.Message)
			return
		}
	}
}

// HTTPJobExecutor this will execute an http job
func (jobProcessor *JobProcessor) HTTPJobExecutor(jobTransformer *transformers.Job, executionManger *execution.Manager) func() {
	return func() {
		jobProcessor.PendingJobs <- &PendingJob{
			Job: jobTransformer,
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
		jobManager := job.Manager{}

		jobsTotalCount, err := jobManager.GetJobsTotalCountByProjectUUID(jobProcessor.DBConnection, projectTransformer.UUID)
		if err != nil {
			panic(err)
		}

		utils.Info(fmt.Sprintf("Total number of jobs for project %v is %v : ", projectTransformer.ID, jobsTotalCount))
		paginatedJobTransformers, err := jobService.GetJobsByProjectUUID(projectTransformer.UUID, 0, jobsTotalCount, "date_created")

		jobTransformers := []transformers.Job{}

		for _, jobTransformer := range paginatedJobTransformers.Data {
			jobTransformers = append(jobTransformers, jobTransformer)
		}

		jobProcessor.RecoverJobExecutions(jobTransformers)

		utils.Info(fmt.Sprintf("Recovered %v Jobs for Project with ID: %v",
			len(jobProcessor.RecoveredJobs),
			projectTransformer.ID))

		for _, jobTransformer := range paginatedJobTransformers.Data {
			wg.Add(1)
			utils.Info(fmt.Sprintf("Adding job %v of project %v : ", jobTransformer.ID, projectTransformer.ID))
			go jobProcessor.AddJob(jobTransformer, &wg)
		}
		wg.Wait()
	}

	jobProcessor.Cron.Start()

	go func() {
		for {
			select {
			case pendingJob := <- jobProcessor.PendingJobs:
				jobProcessor.ExecuteHTTPJob(pendingJob.Job, pendingJob.Execution)
			}
		}
	}()
}

// AddJob adds a single job to the queue
func (jobProcessor *JobProcessor) AddJob(jobTransformer transformers.Job, wg *sync.WaitGroup) {
	// TODO: Check if we're within resource usage before adding another job
	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()

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
	_, createErr := executionManager.CreateOne(jobProcessor.DBConnection)
	if createErr != nil {
		fmt.Println("Error Getting Execution", utils.Error(createErr.Message))
		return
	}

	cronAddJobErr := jobProcessor.Cron.AddFunc(jobTransformer.Spec, jobProcessor.HTTPJobExecutor(&jobTransformer, &executionManager))
	if cronAddJobErr != nil {
		fmt.Println("Error Add Cron JOb", cronAddJobErr.Error())
		return
	}
}
