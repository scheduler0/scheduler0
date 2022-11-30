package job_executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/araddon/dateparse"
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
	"strconv"
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
		jobExecutor.logger.Println(fmt.Sprintf("batch query error:: %s", batchGetError.Message))
		return
	}

	getPendingJob := func(jobID int64) *models.JobProcess {
		for _, pendingJob := range pendingJobs {
			if pendingJob.Job.ID == jobID {
				return pendingJob
			}
		}
		return nil
	}

	jobExecutor.logger.Println(fmt.Sprintf("batched queried %v", len(jobs)))

	jobsToExecute := make([]models.JobModel, 0)

	for _, job := range jobs {
		pendingJob := getPendingJob(job.ID)
		if pendingJob != nil {
			jobsToExecute = append(jobsToExecute, *pendingJob.Job)
		} else {
			for i, jobProcess := range jobExecutor.jobProcess {
				if jobProcess.Job.ID == job.ID {
					jobProcess.Cron.Stop()
					jobExecutor.jobProcess = append(jobExecutor.jobProcess[:i], jobExecutor.jobProcess[i+1:]...)
				}
			}
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
	if leaderAddress == fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port) &&
		len(configuration.Servers) < 2 {
		body := models.JobStateLog{
			State:         actionType,
			Data:          []models.JobModel{},
			ServerAddress: fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port),
		}

		for _, pendingJob := range pendingJobs {
			body.Data = append(body.Data, pendingJob)
		}

		data := []interface{}{}
		data = append(data, body)

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

	err = utils.RetryOnError(func() error {
		client := http.Client{
			// TODO: make this configurable
			Timeout: time.Minute * 15,
		}
		body := models.JobStateLog{
			State:         actionType,
			Data:          pendingJobs,
			ServerAddress: fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port),
		}
		data, err := json.Marshal(body)
		if err != nil {
			jobExecutor.logger.Fatalln("failed to convert jobs ", err)
		}
		reader := bytes.NewReader(data)
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/execution-logs", utils.GetNodeIPWithRaftAddress(jobExecutor.logger, leaderAddress)), reader)
		if err != nil {
			jobExecutor.logger.Fatalln("failed to req ", err)
		}
		req.Header.Set(headers.PeerHeader, "peer")
		req.Header.Set(headers.PeerAddressHeader, fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port))
		credentials := secrets.GetSecrets(jobExecutor.logger)
		req.SetBasicAuth(credentials.AuthUsername, credentials.AuthPassword)

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
		if cronAddJobErr != nil {
			jobExecutor.logger.Println("error adding creating cron job", cronAddJobErr.Error())
			return
		}

		jobProcess.Cron.Start()
		jobExecutor.jobProcess = append(jobExecutor.jobProcess, &jobProcess)
	}
}

func (jobExecutor *JobExecutor) LogPrepare(jobState models.JobStateLog) {
	jobExecutor.fileMtx.Lock()
	defer jobExecutor.fileMtx.Unlock()
	jobProcesses := []*models.JobProcess{}

	for _, job := range jobState.Data {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(job.LastExecutionDate)

		entry := fmt.Sprintf("prepare, %v, %v, %v, %v, %v", jobState.ServerAddress, job.ID, job.ExecutionId, job.LastExecutionDate.String(), executionTime.String())
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

func (jobExecutor *JobExecutor) LogCommit(jobState models.JobStateLog) {
	jobExecutor.fileMtx.Lock()
	defer jobExecutor.fileMtx.Unlock()
	for _, job := range jobState.Data {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(job.LastExecutionDate)
		entry := fmt.Sprintf("commit, %v, %v, %v, %v, %v", jobState.ServerAddress, job.ID, job.ExecutionId, job.LastExecutionDate.String(), executionTime.String())
		jobExecutor.WriteJobExecutionLog(entry)
		for _, jobProcess := range jobExecutor.jobProcess {
			if jobProcess.Job.ID == job.ID {
				jobProcess.Job.LastExecutionDate = executionTime
			}
		}
	}
}

func (jobExecutor *JobExecutor) LogErrors(jobState models.JobStateLog) {
	jobExecutor.fileMtx.Lock()
	defer jobExecutor.fileMtx.Unlock()
	for _, job := range jobState.Data {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(job.LastExecutionDate)
		entry := fmt.Sprintf("error, %v, %v, %v, %v, %v", jobState.ServerAddress, job.ID, job.ExecutionId, job.LastExecutionDate.String(), executionTime.String())
		jobExecutor.WriteJobExecutionLog(entry)
		for _, jobProcess := range jobExecutor.jobProcess {
			if jobProcess.Job.ID == job.ID {
				jobProcess.Job.LastExecutionDate = executionTime
			}
		}
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

func (jobExecutor *JobExecutor) GetJobLastCommandLog(state string, jobId int64) (models.JobStateLog, bool) {
	data, err := io.ReadAll(jobExecutor.logFile)
	lines := []string{}

	fileData := string(data)
	if err == nil {
		lines = strings.Split(fileData, "\n")
	}

	lastJobStatLog := models.JobStateLog{
		Data: []models.JobModel{
			{
				ID: jobId,
			},
		},
	}

	found := false

	for _, line := range lines {
		tokens := strings.Split(line, ",")
		commandToken := strings.Trim(tokens[0], " ")
		serverAddressToken := strings.Trim(tokens[1], " ")
		jobIdToken := strings.Trim(tokens[2], " ")
		executionIdToken := strings.Trim(tokens[3], " ")
		lastExecutionTimeToken := strings.Trim(tokens[4], " ")
		executionTimeToken := strings.Trim(tokens[5], " ")
		jobIdInt, convertErr := strconv.Atoi(jobIdToken)
		if convertErr != nil {
			jobExecutor.logger.Fatalln("failed to convert job id to integer")
		}
		if int64(jobIdInt) == jobId && commandToken == state {
			executionTime, errParse := dateparse.ParseLocal(executionTimeToken)
			if errParse != nil {
				jobExecutor.logger.Fatalln("failed to convert job id to integer")
			}

			lastExecutionTime, errParse := dateparse.ParseLocal(lastExecutionTimeToken)
			if errParse != nil {
				jobExecutor.logger.Fatalln("failed to convert job id to integer")
			}

			if executionTime.After(lastJobStatLog.ExecutionTime) {
				lastJobStatLog.ExecutionTime = executionTime
				lastJobStatLog.ServerAddress = serverAddressToken
				lastJobStatLog.Data[0].LastExecutionDate = lastExecutionTime
				lastJobStatLog.Data[0].ExecutionId = executionIdToken
				found = true
			}
		}
	}

	switch state {
	case "prepare":
		lastJobStatLog.State = constants.CommandTypePrepareJobExecutions
		break
	case "commit":
		lastJobStatLog.State = constants.CommandTypeCommitJobExecutions
		break
	case "error":
		lastJobStatLog.State = constants.CommandTypeErrorJobExecutions
		break
	}

	return lastJobStatLog, found
}

func (jobExecutor *JobExecutor) GetJobLogsForServer(serverAddress string) map[int]*models.JobStateLog {
	data, err := io.ReadAll(jobExecutor.logFile)

	lines := []string{}
	fileData := string(data)
	if err == nil {
		lines = strings.Split(fileData, "\n")
	}

	results := map[int]*models.JobStateLog{}

	for _, line := range lines {
		if len(line) < 1 {
			continue
		}
		tokens := strings.Split(line, ",")
		stateToken := strings.Trim(tokens[0], " ")
		serverAddressToken := strings.Trim(tokens[1], " ")
		jobIdToken := strings.Trim(tokens[2], " ")
		executionIdToken := strings.Trim(tokens[3], " ")
		lastExecutionTimeToken := strings.Trim(tokens[4], " ")
		executionTimeToken := strings.Trim(tokens[5], " ")
		jobIdInt, convertErr := strconv.Atoi(jobIdToken)

		if serverAddressToken != serverAddress {
			continue
		}

		if convertErr != nil {
			jobExecutor.logger.Fatalln("failed to convert job id to integer")
		}

		if _, ok := results[jobIdInt]; !ok {
			jobState := &models.JobStateLog{
				ServerAddress: serverAddress,
				Data: []models.JobModel{
					{
						ID: int64(jobIdInt),
					},
				},
			}
			results[jobIdInt] = jobState
		}

		jobState := results[jobIdInt]

		executionTime, errParse := dateparse.ParseLocal(executionTimeToken)
		if errParse != nil {
			jobExecutor.logger.Fatalln("failed to convert job id to integer")
		}

		lastExecutionTime, errParse := dateparse.ParseLocal(lastExecutionTimeToken)
		if errParse != nil {
			jobExecutor.logger.Fatalln("failed to convert job id to integer")
		}

		if executionTime.After(jobState.ExecutionTime) || jobState.ExecutionTime.IsZero() {
			jobState.ExecutionTime = executionTime
			jobState.Data[0].LastExecutionDate = lastExecutionTime
			jobState.Data[0].ExecutionId = executionIdToken
			switch stateToken {
			case "prepare":
				jobState.State = constants.CommandTypePrepareJobExecutions
				break
			case "commit":
				jobState.State = constants.CommandTypeCommitJobExecutions
				break
			case "error":
				jobState.State = constants.CommandTypeErrorJobExecutions
				break
			}
		}
	}

	return results
}

func (jobExecutor *JobExecutor) AddNewProcess(job models.JobModel) *models.JobProcess {
	jobProcess := models.JobProcess{
		Job:  &job,
		Cron: cron.New(),
	}

	jobExecutor.logger.Println("scheduling job with id", jobProcess.Job.ID)

	cronAddJobErr := jobProcess.Cron.AddFunc(jobProcess.Job.Spec, jobExecutor.AddPendingJobToChannel(&jobProcess))
	if cronAddJobErr != nil {
		jobExecutor.logger.Println("error adding creating cron job", cronAddJobErr.Error())
		return nil
	}

	jobProcess.Cron.Start()
	jobExecutor.jobProcess = append(jobExecutor.jobProcess, &jobProcess)

	return &jobProcess
}
