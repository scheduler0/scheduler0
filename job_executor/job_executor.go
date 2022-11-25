package job_executor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/raft"
	"github.com/robfig/cron"
	"github.com/spf13/afero"
	"log"
	"net/http"
	"os"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/executor"
	"scheduler0/headers"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/secrets"
	"scheduler0/utils"
	"strings"
	"sync"
	"time"
)

type JobExecutor struct {
	mtx           sync.Mutex
	fileMtx       sync.Mutex
	raft          *raft.Raft
	LeaderAddress string
	PendingJobs   chan *models.JobProcess
	jobRepo       repository.Job
	logger        *log.Logger
	jobProcess    []*models.JobProcess
}

func NewJobExecutor(logger *log.Logger, jobRepository repository.Job) *JobExecutor {
	return &JobExecutor{
		PendingJobs: make(chan *models.JobProcess, 100),
		jobRepo:     jobRepository,
		logger:      logger,
	}
}

// AddPendingJobToChannel this will execute a http job
func (jobExecutor *JobExecutor) AddPendingJobToChannel(jobProcess *models.JobProcess) func() {
	return func() {
		jobExecutor.PendingJobs <- jobProcess
	}
}

// ExecutePendingJobs executes and http job
func (jobExecutor *JobExecutor) ExecutePendingJobs(pendingJobs []*models.JobProcess) {
	jobIDs := make([]int64, 0)
	for _, pendingJob := range pendingJobs {
		jobIDs = append(jobIDs, pendingJob.Job.ID)
	}

	jobs, batchGetError := jobExecutor.jobRepo.BatchGetJobsByID(jobIDs)
	if batchGetError != nil {
		jobExecutor.logger.Println(fmt.Sprintf("Batch Query Error:: %s", batchGetError.Message))
		return
	}

	// TODO: find jobs that are being processed but have been deleted

	getPendingJob := func(jobID int64) *models.JobProcess {
		for _, pendingJob := range pendingJobs {
			if pendingJob.Job.ID == jobID {
				return pendingJob
			}
		}
		return nil
	}

	jobExecutor.logger.Println(fmt.Sprintf("Batched Queried %v", len(jobs)))

	jobsToExecute := make([]*models.JobModel, 0)

	// TODO: execute jobs based on priority level

	for _, job := range jobs {
		pendingJob := getPendingJob(job.ID)
		if pendingJob != nil {
			jobsToExecute = append(jobsToExecute, pendingJob.Job)
		}
	}

	onSuccess := func(pendingJobs []*models.JobModel) {
		for _, pendingJob := range pendingJobs {
			jobExecutor.logger.Println(fmt.Sprintf("Executed job %v", pendingJob.ID))
		}
		jobExecutor.LogJobExecutionStateOnLeader(pendingJobs, constants.CommandTypeCommitJobExecutions)
	}

	onFail := func(erroredJobs []*models.JobModel, err error) {
		jobExecutor.LogJobExecutionStateOnLeader(erroredJobs, constants.CommandTypeErrorJobExecutions)
		jobExecutor.logger.Println(fmt.Sprintf("%v jobs failed"), err.Error())
	}

	executorService := executor.NewService(jobExecutor.logger, jobsToExecute, onSuccess, onFail)

	// TODO: execute job based on job execution type
	executorService.ExecuteHTTP()
}

// ListenToChannelsUpdates periodically checks channels for updates
func (jobExecutor *JobExecutor) ListenToChannelsUpdates() {
	pendingJobs := make([]*models.JobProcess, 0)
	ticker := time.NewTicker(time.Millisecond * 100)

	for {
		select {
		case pendingJob := <-jobExecutor.PendingJobs:
			pendingJobs = append(pendingJobs, pendingJob)
		case <-ticker.C:
			if len(pendingJobs) > 0 {
				prepareJobs := make([]*models.JobModel, 0)
				for _, jobProcess := range pendingJobs[0:] {
					prepareJobs = append(prepareJobs, jobProcess.Job)
				}
				jobExecutor.LogJobExecutionStateOnLeader(prepareJobs, constants.CommandTypePrepareJobExecutions)
				jobExecutor.logger.Println(fmt.Sprintf("%v Pending Jobs To Execute", len(pendingJobs[0:])))
				pendingJobs = pendingJobs[len(pendingJobs):]
			}
		}
	}
}

func (jobExecutor *JobExecutor) LogJobExecutionStateOnLeader(pendingJobs []*models.JobModel, actionType constants.Command) {
	client := http.Client{}
	body := models.JobStateReqPayload{
		State: actionType,
		Data:  pendingJobs,
	}
	data, err := json.Marshal(body)
	if err != nil {
		jobExecutor.logger.Fatalln("failed to convert jobs ", err)
	}
	reader := bytes.NewReader(data)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/execution-logs", jobExecutor.LeaderAddress), reader)

	configs := config.GetScheduler0Configurations(jobExecutor.logger)

	req.Header.Set(headers.PeerHeader, "peer")
	req.Header.Set(headers.PeerAddressHeader, fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port))
	credentials := secrets.GetSecrets(jobExecutor.logger)
	req.SetBasicAuth(credentials.AuthUsername, credentials.AuthPassword)
	err = utils.RetryOnError(func() error {
		res, err := client.Do(req)
		if err != nil {
			return err
		}

		if res.StatusCode != http.StatusOK {
			return errors.New("failed to send job log state to leader")
		}

		return nil
	}, configs.JobExecutionStateLogRetryMax, configs.JobExecutionStateLogRetryDelay)
	if err != nil {
		jobExecutor.logger.Fatalln("failed to send request ", err)
	}
}

func (jobExecutor *JobExecutor) Run(jobs []models.JobModel) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	for i, _ := range jobs {
		jobProcess := models.JobProcess{
			Job:  &jobs[i],
			Cron: cron.New(),
		}

		jobExecutor.logger.Println("scheduling job with id", jobProcess.Job.ID)

		cronAddJobErr := jobProcess.Cron.AddFunc(jobProcess.Job.Spec, jobExecutor.AddPendingJobToChannel(&jobProcess))

		jobProcess.Cron.Start()
		jobExecutor.jobProcess = append(jobExecutor.jobProcess, &jobProcess)

		if cronAddJobErr != nil {
			jobExecutor.logger.Println("Error Add Cron JOb", cronAddJobErr.Error())
			return
		}
	}
}

func (jobExecutor *JobExecutor) LogPrepare(jobs []models.JobModel) {
	jobExecutor.fileMtx.Lock()
	defer jobExecutor.fileMtx.Unlock()
	jobProcesses := []*models.JobProcess{}

	for _, job := range jobs {
		entry := fmt.Sprintf("prepare %v %v", job.ID, time.Now().UTC())
		WriteJobExecutionLog(job, entry)
		for _, jobProcess := range jobExecutor.jobProcess {
			if jobProcess.Job.ID == job.ID {
				jobProcesses = append(jobProcesses, jobProcess)
			}
		}
	}

	jobExecutor.ExecutePendingJobs(jobProcesses)
}

func (jobExecutor *JobExecutor) LogCommit(jobs []models.JobModel) {
	jobExecutor.fileMtx.Lock()
	defer jobExecutor.fileMtx.Unlock()
	for _, job := range jobs {
		entry := fmt.Sprintf("commit %v %v", job.ID, time.Now().UTC())
		WriteJobExecutionLog(job, entry)
	}
}

func (jobExecutor *JobExecutor) LogErrors(jobs []models.JobModel) {
	jobExecutor.fileMtx.Lock()
	defer jobExecutor.fileMtx.Unlock()
	for _, job := range jobs {
		entry := fmt.Sprintf("error %v %v", job.ID, time.Now().UTC())
		WriteJobExecutionLog(job, entry)
	}
}

func WriteJobExecutionLog(job models.JobModel, entry string) {
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

	logs = append(logs, entry)
	logs = append(logs, lines...)

	str := strings.Join(logs, "\n")
	sliceByte := []byte(str)

	writeErr := afero.WriteFile(fs, logFilePath, sliceByte, os.ModePerm)
	if writeErr != nil {
		log.Fatalln("Binary Write Error::", writeErr)
	}
}
