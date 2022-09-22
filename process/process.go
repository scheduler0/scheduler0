package process

import (
	"database/sql"
	"fmt"
	"github.com/robfig/cron"
	"github.com/spf13/afero"
	"log"
	"os"
	"scheduler0/constants"
	"scheduler0/executor"
	"scheduler0/models"
	repository2 "scheduler0/repository"
	"scheduler0/utils"
	"strings"
	"sync"
	"time"
)

// JobProcessor handles executions of jobs
type JobProcessor struct {
	Cron              *cron.Cron
	RecoveredJobs     []RecoveredJob
	PendingJobs       chan *models.JobModel
	DBConnection      *sql.DB
	DBMu              *sync.Mutex
	PendingJobUpdates chan *models.JobModel
	fnQueue           chan func(db *sql.DB)
	jobRepo           repository2.Job
	projectRepo       repository2.Project
}

// NewJobProcessor creates a new job processor
func NewJobProcessor(dbConnection *sql.DB, jobRepo repository2.Job, projectRepo repository2.Project) *JobProcessor {
	return &JobProcessor{
		DBConnection:      dbConnection,
		Cron:              cron.New(),
		RecoveredJobs:     []RecoveredJob{},
		DBMu:              &sync.Mutex{},
		PendingJobs:       make(chan *models.JobModel, 100),
		PendingJobUpdates: make(chan *models.JobModel, 100),
		jobRepo:           jobRepo,
		projectRepo:       projectRepo,
	}
}

// RecoverJobExecutions find jobs that could've not been executed due to timeout
//func (jobProcessor *JobProcessor) RecoverJobExecutions(jobTransformers []models.JobModel) {
//	manager := repository.ExecutionRepo{}
//
//	//dir, err := os.Getwd()
//	//if err != nil {
//	//	log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
//	//}
//	//dbFilePath := fmt.Sprintf("%v/.scheduler0.execs", dir)
//	//execsLogFile, err := os.ReadFile(dbFilePath)
//	//
//	//buf := &bytes.Buffer{}
//	//encodeErr := gob.NewEncoder(buf).Encode(execsLogFile)
//	//if encodeErr != nil {
//	//	log.Fatalln(fmt.Errorf("Fatal encode err: %s \n", encodeErr))
//	//}
//	//bs := buf.String()
//	//
//	//fmt.Println("bs", bs)
//
//	for _, jobTransformer := range jobTransformers {
//		if jobProcessor.IsRecovered(jobTransformer.ID) {
//			continue
//		}
//		count, err, executionManagers := manager.FindJobExecutionPlaceholderByID(jobProcessor.DBConnection, jobTransformer.ID)
//		if err != nil {
//			utils.Error(fmt.Sprintf("Error occurred while fetching execution mangers for jobs to be recovered %s", err.Message))
//			continue
//		}
//
//		if count < 1 {
//			continue
//		}
//
//		executionManager := executionManagers[0]
//		schedule, parseErr := cron.Parse(jobTransformer.Spec)
//
//		if parseErr != nil {
//			utils.Error(fmt.Sprintf("Failed to create schedule%s", parseErr.Error()))
//			continue
//		}
//		now := time.Now().UTC()
//		executionTime := schedule.Next(executionManager.TimeAdded).UTC()
//
//		if now.Before(executionTime) {
//			executionTransformer := transformers.Execution{}
//			executionTransformer.FromRepo(executionManager)
//			recovery := RecoveredJob{
//				Execution: &executionTransformer,
//				Job:       &jobTransformer,
//			}
//			jobProcessor.RecoveredJobs = append(jobProcessor.RecoveredJobs, recovery)
//		}
//	}
//}

// IsRecovered Check if a job is in recovered job queues
func (jobProcessor *JobProcessor) IsRecovered(jobID int64) bool {
	for _, recoveredJob := range jobProcessor.RecoveredJobs {
		if recoveredJob.Job.ID == jobID {
			return true
		}
	}

	return false
}

// GetRecovery Returns recovery object
func (jobProcessor *JobProcessor) GetRecovery(jobID int64) *RecoveredJob {
	for _, recoveredJob := range jobProcessor.RecoveredJobs {
		if recoveredJob.Job.ID == jobID {
			return &recoveredJob
		}
	}

	return nil
}

// RemoveJobRecovery Removes a recovery object
func (jobProcessor *JobProcessor) RemoveJobRecovery(jobID int64) {
	jobIndex := -1

	for index, recoveredJob := range jobProcessor.RecoveredJobs {
		if recoveredJob.Job.ID == jobID {
			jobIndex = index
			break
		}
	}

	jobProcessor.RecoveredJobs = append(jobProcessor.RecoveredJobs[:jobIndex], jobProcessor.RecoveredJobs[jobIndex+1:]...)
}

// ExecutePendingJobs executes and http job
func (jobProcessor *JobProcessor) ExecutePendingJobs(pendingJobs []models.JobModel) {
	jobIDs := make([]int64, 0)
	for _, pendingJob := range pendingJobs {
		jobIDs = append(jobIDs, pendingJob.ID)
	}

	jobs, batchGetError := jobProcessor.jobRepo.BatchGetJobsByID(jobIDs)
	if batchGetError != nil {
		utils.Error(fmt.Sprintf("Batch Query Error:: %s", batchGetError.Message))
	}

	getPendingJob := func(jobID int64) *models.JobModel {
		for _, pendingJob := range pendingJobs {
			if pendingJob.ID == jobID {
				return &pendingJob
			}
		}
		return nil
	}

	utils.Info(fmt.Sprintf("Batched Queried %v", len(jobs)))

	jobsToExecute := make([]*models.JobModel, 0)

	for _, job := range jobs {
		pendingJob := getPendingJob(job.ID)
		if pendingJob != nil {
			jobsToExecute = append(jobsToExecute, pendingJob)
		}
	}

	onSuccess := func(pendingJobs []*models.JobModel) {
		for _, pendingJob := range pendingJobs {
			go jobProcessor.WriteJobExecutionLog(*pendingJob)

			if jobProcessor.IsRecovered(pendingJob.ID) {
				jobProcessor.RemoveJobRecovery(pendingJob.ID)
				jobProcessor.AddJobs([]models.JobModel{*pendingJob}, nil)
			}
			utils.Info(fmt.Sprintf("Executed job %v", pendingJob.ID))
		}
	}

	onFail := func(pj []*models.JobModel, err error) {
		utils.Error("HTTP REQUEST ERROR:: ", err.Error())
	}

	executorService := executor.NewService(jobsToExecute, onSuccess, onFail)
	executorService.ExecuteHTTP()
}

// AddPendingJobToChannel this will execute a http job
func (jobProcessor *JobProcessor) AddPendingJobToChannel(jobTransformer models.JobModel) func() {
	return func() {
		jobProcessor.PendingJobs <- &jobTransformer
	}
}

// StartJobs the cron job process
func (jobProcessor *JobProcessor) StartJobs() {
	totalProjectCount, countErr := jobProcessor.projectRepo.Count()
	if countErr != nil {
		log.Fatalln(countErr.Message)
	}

	utils.Info("Total number of projects: ", totalProjectCount)

	projectTransformers, listErr := jobProcessor.projectRepo.List(0, totalProjectCount)
	if listErr != nil {
		log.Fatalln(countErr.Message)
	}

	var wg sync.WaitGroup

	for _, projectTransformer := range projectTransformers {
		wg.Add(1)

		jobsTotalCount, err := jobProcessor.jobRepo.GetJobsTotalCountByProjectID(projectTransformer.ID)
		if err != nil {
			log.Fatalln(err.Message)
		}

		utils.Info(fmt.Sprintf("Total number of jobs for project %v is %v : ", projectTransformer.ID, jobsTotalCount))
		paginatedJobTransformers, _, loadErr := jobProcessor.jobRepo.GetJobsPaginated(projectTransformer.ID, 0, jobsTotalCount)
		if loadErr != nil {
			log.Fatalln(loadErr.Message)
		}

		jobTransformers := make([]models.JobModel, 0)

		for _, jobTransformer := range paginatedJobTransformers {
			jobTransformers = append(jobTransformers, jobTransformer)
		}

		//jobProcessor.RecoverJobExecutions(jobTransformers)

		utils.Info(fmt.Sprintf("Recovered %v Jobs for Project with ID: %v",
			len(jobProcessor.RecoveredJobs),
			projectTransformer.ID))

		go jobProcessor.AddJobs(paginatedJobTransformers, &wg)

		wg.Wait()
	}

	jobProcessor.Cron.Start()

	go jobProcessor.ListenToChannelsUpdates()
}

func (jobProcessor *JobProcessor) WriteJobExecutionLog(job models.JobModel) {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	dirPath := fmt.Sprintf("%v/%v", dir, constants.ExecutionLogsDir)
	logFilePath := fmt.Sprintf("%v/%v/%v.txt", dir, constants.ExecutionLogsDir, job.ID)

	fs := afero.NewOsFs()

	exists, err := afero.DirExists(fs, dirPath)
	if err != nil {
		return
	}

	if !exists {
		err := fs.Mkdir(dirPath, os.ModePerm)
		if err != nil {
			return
		}
	}

	logs := []string{}
	lines := []string{}

	fileData, err := afero.ReadFile(fs, logFilePath)
	if err == nil {
		dataString := string(fileData)
		lines = strings.Split(dataString, "\n")
	}

	logStr := fmt.Sprintf("execute %v, %v", job.ID, time.Now().UTC())
	logs = append(logs, logStr)
	logs = append(logs, lines...)

	str := strings.Join(logs, "\n")
	sliceByte := []byte(str)

	writeErr := afero.WriteFile(fs, logFilePath, sliceByte, os.ModePerm)
	if writeErr != nil {
		log.Fatalln("Binary Write Error::", writeErr)
	}

}

// ListenToChannelsUpdates periodically checks channels for updates
func (jobProcessor *JobProcessor) ListenToChannelsUpdates() {
	pendingJobs := make([]models.JobModel, 0)
	ticker := time.NewTicker(time.Millisecond * 100)

	for {
		select {
		case pendingJob := <-jobProcessor.PendingJobs:
			pendingJobs = append(pendingJobs, *pendingJob)
		case <-ticker.C:
			if len(pendingJobs) > 0 {
				jobProcessor.ExecutePendingJobs(pendingJobs[0:])
				utils.Info(fmt.Sprintf("%v Pending Jobs To Execute", len(pendingJobs[0:])))
				pendingJobs = pendingJobs[len(pendingJobs):]
			}
		}
	}
}

// AddJobs adds a single job to the queue
func (jobProcessor *JobProcessor) AddJobs(jobTransformers []models.JobModel, wg *sync.WaitGroup) {
	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()

	for _, jobTransformer := range jobTransformers {
		if recovery := jobProcessor.GetRecovery(jobTransformer.ID); recovery != nil {
			go recovery.Run(jobProcessor)
			return
		}
	}

	for _, jobTransformer := range jobTransformers {
		cronAddJobErr := jobProcessor.Cron.AddFunc(jobTransformer.Spec, jobProcessor.AddPendingJobToChannel(jobTransformer))
		if cronAddJobErr != nil {
			utils.Error("Error Add Cron JOb", cronAddJobErr.Error())
			return
		}
	}

	utils.Info(fmt.Sprintf("Queued %v Jobs", len(jobTransformers)))
}
