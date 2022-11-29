package job_executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/raft"
	"github.com/robfig/cron"
	"io"
	"log"
	"net/http"
	"os"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/executors"
	"scheduler0/fsm"
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
	context     context.Context
	mtx         sync.Mutex
	fileMtx     sync.Mutex
	Raft        *raft.Raft
	PendingJobs chan *models.JobProcess
	jobRepo     repository.Job
	logFile     *os.File
	logger      *log.Logger
	jobProcess  []*models.JobProcess
	cancelReq   context.CancelFunc
	executor    *executors.Service
}

func NewJobExecutor(logger *log.Logger, jobRepository repository.Job) *JobExecutor {
	ctx, cancel := context.WithCancel(context.Background())
	dir, err := os.Getwd()
	if err != nil {
		logger.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}
	dirPath := fmt.Sprintf("%v/%v", dir, constants.ExecutionLogsDir)
	mkdirErr := os.Mkdir(dirPath, os.ModePerm)
	if !os.IsExist(mkdirErr) && !os.IsNotExist(mkdirErr) && err != nil {
		logger.Fatalln(fmt.Errorf("failed to create the logs dir %s \n", err))
	}
	logFilePath := fmt.Sprintf("%v/executions.txt", dirPath)
	logFile, err := os.OpenFile(logFilePath, os.O_RDWR, os.ModePerm)
	if os.IsNotExist(err) {
		file, createErr := os.Create(logFilePath)
		if createErr != nil {
			logger.Fatalln(fmt.Errorf("failed to create the executions.txt file %s \n", err))
		}
		logFile = file
	}
	return &JobExecutor{
		PendingJobs: make(chan *models.JobProcess, 100),
		jobRepo:     jobRepository,
		logger:      logger,
		context:     ctx,
		cancelReq:   cancel,
		logFile:     logFile,
		executor:    executors.NewService(logger),
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

	jobsToExecute := make([]models.JobModel, 0)

	// TODO: execute jobs based on priority level

	for _, job := range jobs {
		pendingJob := getPendingJob(job.ID)
		if pendingJob != nil {
			jobsToExecute = append(jobsToExecute, *pendingJob.Job)
		}
	}

	// TODO: execute job based on job execution type
	if len(jobsToExecute) > 0 {
		jobExecutor.executor.ExecuteHTTP(
			jobsToExecute,
			jobExecutor.context,
			jobExecutor.HandleSuccessJobs,
			jobExecutor.HandleFailedJobs,
		)
	}
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
				prepareJobs := make([]models.JobModel, 0)
				for _, jobProcess := range pendingJobs[0:] {
					executionUUID, err := uuid.GenerateUUID()
					if err != nil {
						jobExecutor.logger.Fatalln(fmt.Sprintf("failed to created execution id for job %s", err.Error()))
					}
					jobProcess.Job.ExecutionId = executionUUID
					prepareJobs = append(prepareJobs, *jobProcess.Job)
				}
				jobExecutor.LogJobExecutionStateOnLeader(prepareJobs, constants.CommandTypePrepareJobExecutions)
				jobExecutor.logger.Println(fmt.Sprintf("%v Pending Jobs To Execute", len(pendingJobs[0:])))
				pendingJobs = pendingJobs[len(pendingJobs):]
			}
		}
	}
}

func (jobExecutor *JobExecutor) LogJobExecutionStateOnLeader(pendingJobs []models.JobModel, actionType constants.Command) {
	configs := config.GetScheduler0Configurations(jobExecutor.logger)
	configuration := jobExecutor.Raft.GetConfiguration().Configuration()

	leaderAddress := ""

	err := utils.RetryOnError(func() error {
		address, _ := jobExecutor.Raft.LeaderWithID()
		leaderAddress = string(address)
		if leaderAddress == "" {
			return errors.New("cannot get leader with id")
		}
		return nil
	}, configs.JobExecutionStateLogRetryMax, configs.JobExecutionStateLogRetryDelay)

	if err != nil {
		jobExecutor.logger.Fatalln("cannot get leader with id")
	}

	// When only a single-node is running
	if string(leaderAddress) == fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port) &&
		len(configuration.Servers) < 2 {
		data := []interface{}{}

		for _, pendingJob := range pendingJobs {
			data = append(data, pendingJob)
		}

		_, applyErr := fsm.AppApply(
			jobExecutor.logger,
			jobExecutor.Raft,
			actionType,
			configs.RaftAddress,
			data,
		)

		if applyErr != nil {
			jobExecutor.logger.Fatalln("failed to apply job update states ", applyErr)
		}

		return
	}

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
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/execution-logs", utils.GetNodeIPWithRaftAddress(jobExecutor.logger, string(leaderAddress))), reader)
	if err != nil {
		jobExecutor.logger.Fatalln("failed to req ", err)
	}
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
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(job.LastExecutionDate)

		entry := fmt.Sprintf("prepare %v, %v, %v, %v", job.ID, job.ExecutionId, job.LastExecutionDate.String(), executionTime.String())
		jobExecutor.WriteJobExecutionLog(entry)
		for _, jobProcess := range jobExecutor.jobProcess {
			if jobProcess.Job.ID == job.ID {
				jobProcesses = append(jobProcesses, jobProcess)
			}
		}
	}

	if len(jobProcesses) > 0 {
		jobExecutor.ExecutePendingJobs(jobProcesses)
	}
}

func (jobExecutor *JobExecutor) LogCommit(jobs []models.JobModel) {
	jobExecutor.fileMtx.Lock()
	defer jobExecutor.fileMtx.Unlock()
	for _, job := range jobs {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(job.LastExecutionDate)
		entry := fmt.Sprintf("commit %v, %v, %v, %v", job.ID, job.ExecutionId, job.LastExecutionDate.String(), executionTime.String())
		// TODO: update last execution data of job in memory
		jobExecutor.WriteJobExecutionLog(entry)
	}
}

func (jobExecutor *JobExecutor) LogErrors(jobs []models.JobModel) {
	jobExecutor.fileMtx.Lock()
	defer jobExecutor.fileMtx.Unlock()
	for _, job := range jobs {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(job.LastExecutionDate)
		entry := fmt.Sprintf("error %v, %v, %v, %v", job.ID, job.ExecutionId, job.LastExecutionDate.String(), executionTime.String())
		// TODO: update last execution data of job in memory
		jobExecutor.WriteJobExecutionLog(entry)
	}
}

func (jobExecutor *JobExecutor) StopAll() {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	if len(jobExecutor.jobProcess) < 1 {
		return
	}

	for _, jobProcess := range jobExecutor.jobProcess {
		jobProcess.Cron.Stop()
	}

	jobExecutor.logger.Println("stopped all scheduled job")
	jobExecutor.PendingJobs = make(chan *models.JobProcess, 100)
	jobExecutor.jobProcess = []*models.JobProcess{}
	jobExecutor.cancelReq()
	ctx, cancel := context.WithCancel(context.Background())
	jobExecutor.cancelReq = cancel
	jobExecutor.context = ctx
}

func (jobExecutor *JobExecutor) HandleSuccessJobs(successfulJobs []models.JobModel) {
	for _, pendingJob := range successfulJobs {
		jobExecutor.logger.Println(fmt.Sprintf("Executed job %v", pendingJob.ID))
	}
	jobExecutor.LogJobExecutionStateOnLeader(successfulJobs, constants.CommandTypeCommitJobExecutions)
}

func (jobExecutor *JobExecutor) HandleFailedJobs(erroredJobs []models.JobModel) {
	jobExecutor.LogJobExecutionStateOnLeader(erroredJobs, constants.CommandTypeErrorJobExecutions)
}

func (jobExecutor *JobExecutor) WriteJobExecutionLog(entry string) {
	data, err := io.ReadAll(jobExecutor.logFile)

	logs := []string{}
	lines := []string{}

	fileData := string(data)
	if err == nil {
		lines = strings.Split(fileData, "\n")
	}

	logs = append(logs, entry)
	logs = append(logs, lines...)

	str := strings.Join(logs, "\n")
	sliceByte := []byte(str)

	_, writeErr := jobExecutor.logFile.Write(sliceByte)
	if writeErr != nil {
		log.Fatalln("execution log write error::", writeErr)
	}
}
