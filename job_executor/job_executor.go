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
	"math"
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
	PendingJobs chan models.JobModel
	jobRepo     repository.Job
	logFile     *os.File
	logger      *log.Logger
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
		PendingJobs: make(chan models.JobModel, constants.JobMaxBatchSize),
		jobRepo:     jobRepository,
		logger:      logger,
		context:     ctx,
		cancelReq:   cancel,
		logFile:     logFile,
		executor:    executors.NewService(logger),
	}
}

// ExecutePendingJobs executes and http job
func (jobExecutor *JobExecutor) ExecutePendingJobs(pendingJobs []models.JobModel) {
	jobIDs := make([]int64, 0)
	for _, pendingJob := range pendingJobs {
		jobIDs = append(jobIDs, pendingJob.ID)
	}

	jobs, batchGetError := jobExecutor.jobRepo.BatchGetJobsByID(jobIDs)
	if batchGetError != nil {
		jobExecutor.logger.Println(fmt.Sprintf("batch query error:: %s", batchGetError.Message))
		return
	}

	getPendingJob := func(jobID int64) *models.JobModel {
		for _, pendingJob := range pendingJobs {
			if pendingJob.ID == jobID {
				return &pendingJob
			}
		}
		return nil
	}

	jobExecutor.logger.Println(fmt.Sprintf("batched queried %v", len(jobs)))

	jobsToExecute := make([]models.JobModel, 0)

	for _, job := range jobs {
		pendingJob := getPendingJob(job.ID)
		if pendingJob != nil {
			jobsToExecute = append(jobsToExecute, *pendingJob)
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
	pendingJobs := make([]models.JobModel, 0)
	ticker := time.NewTicker(time.Millisecond * 100)

	for {
		select {
		case pendingJob := <-jobExecutor.PendingJobs:
			pendingJobs = append(pendingJobs, pendingJob)
		case <-ticker.C:
			if len(pendingJobs) > 0 {
				jobExecutor.ExecutePendingJobs(pendingJobs[:])
				jobExecutor.logger.Println(fmt.Sprintf("%v pending jobs to execute", len(pendingJobs[0:])))
				pendingJobs = pendingJobs[len(pendingJobs):]
			}
		}
	}
}

func (jobExecutor *JobExecutor) LogJobExecutionStateOnLeader(pendingJobs []models.JobModel, actionType constants.Command) {
	configs := config.Configurations(jobExecutor.logger)
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
			Timeout: time.Minute * 99999,
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

func (jobExecutor *JobExecutor) Run(jobsIds []int64) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	lowerBound := jobsIds[0]
	upperBound := jobsIds[1]
	if upperBound-lowerBound > constants.JobMaxBatchSize {
		currentLowerBound := lowerBound
		currentUpperBound := lowerBound + constants.JobMaxBatchSize

		for currentLowerBound < upperBound {

			if utils.MonitorMemoryUsage(jobExecutor.logger) {
				return
			}

			jobExecutor.logger.Println("fetching batching", currentLowerBound, "between", currentUpperBound)
			jobs, getErr := jobExecutor.jobRepo.BatchGetJobsWithIDRange(currentLowerBound, currentUpperBound)

			if getErr != nil {
				jobExecutor.logger.Fatalln("failed to batch get job by ranges ids ", getErr)
			}

			jobExecutor.Schedule(jobs)

			if upperBound-currentUpperBound < constants.JobMaxBatchSize {
				currentLowerBound = currentUpperBound
				currentUpperBound = upperBound
			} else {
				currentLowerBound = currentUpperBound
				currentUpperBound = int64(math.Min(
					float64(currentLowerBound+constants.JobMaxBatchSize),
					float64(upperBound),
				))
			}

		}
	} else {
		jobs, getErr := jobExecutor.jobRepo.BatchGetJobsWithIDRange(lowerBound, upperBound)
		if getErr != nil {
			jobExecutor.logger.Fatalln("failed to batch get job by ranges ids ", getErr)
		}
		jobExecutor.Schedule(jobs)
	}
}

func (jobExecutor *JobExecutor) Schedule(jobs []models.JobModel) {
	if len(jobs) < 1 {
		return
	}
	for i, job := range jobs {
		prepareJobState, prepareFound := jobExecutor.GetJobLastCommandLog("prepare", job.ID)
		commitJobState, commitFound := jobExecutor.GetJobLastCommandLog("commit", job.ID)
		errorJobState, errorFound := jobExecutor.GetJobLastCommandLog("error", job.ID)

		if !prepareFound && !commitFound && !errorFound {
			executionUUID, err := uuid.GenerateUUID()
			if err != nil {
				jobExecutor.logger.Fatalln(fmt.Sprintf("failed to created execution id for job %s", err.Error()))
			}

			jobs[i].LastExecutionDate = job.DateCreated
			jobs[i].ExecutionId = executionUUID
		}

		if prepareFound && !commitFound && !errorFound {
			jobs[i].LastExecutionDate = prepareJobState.Data[0].LastExecutionDate
			jobs[i].ExecutionId = prepareJobState.Data[0].ExecutionId
		}

		if prepareFound && commitFound && !errorFound {
			if prepareJobState.ExecutionTime.After(commitJobState.ExecutionTime) {
				jobs[i].LastExecutionDate = prepareJobState.Data[0].LastExecutionDate
				jobs[i].ExecutionId = prepareJobState.Data[0].ExecutionId
			} else {
				executionUUID, err := uuid.GenerateUUID()
				if err != nil {
					jobExecutor.logger.Fatalln(fmt.Sprintf("failed to created execution id for job %s", err.Error()))
				}

				jobs[i].LastExecutionDate = job.DateCreated
				jobs[i].ExecutionId = executionUUID
			}
		}

		if prepareFound && commitFound && errorFound {
			if prepareJobState.ExecutionTime.After(commitJobState.ExecutionTime) && prepareJobState.ExecutionTime.After(errorJobState.ExecutionTime) {
				jobs[i].LastExecutionDate = prepareJobState.Data[0].LastExecutionDate
				jobs[i].ExecutionId = prepareJobState.Data[0].ExecutionId
			} else {
				executionUUID, err := uuid.GenerateUUID()
				if err != nil {
					jobExecutor.logger.Fatalln(fmt.Sprintf("failed to created execution id for job %s", err.Error()))
				}

				jobs[i].LastExecutionDate = job.DateCreated
				jobs[i].ExecutionId = executionUUID
			}
		}
	}
	//jobExecutor.LogJobExecutionStateOnLeader(jobs, constants.CommandTypePrepareJobExecutions)
	for _, job := range jobs {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(job.LastExecutionDate)
		jobExecutor.AddNewProcess(job, executionTime)
	}

	jobExecutor.logger.Println("scheduled job", jobs[0].ID, "to", jobs[len(jobs)-1].ID)
}

func (jobExecutor *JobExecutor) LogPrepare(jobState models.JobStateLog) {
	jobExecutor.fileMtx.Lock()
	defer jobExecutor.fileMtx.Unlock()
	for _, job := range jobState.Data {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(job.LastExecutionDate)
		entry := fmt.Sprintf("prepare, %v, %v, %v, %v, %v", jobState.ServerAddress, job.ID, job.ExecutionId, job.LastExecutionDate.String(), executionTime.String())
		jobExecutor.WriteJobExecutionLog(entry)
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
	}
}

func (jobExecutor *JobExecutor) StopAll() {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	jobExecutor.logger.Println("stopped all scheduled job")
	jobExecutor.PendingJobs = make(chan models.JobModel)
	jobExecutor.cancelReq()
	ctx, cancel := context.WithCancel(context.Background())
	jobExecutor.cancelReq = cancel
	jobExecutor.context = ctx
}

func (jobExecutor *JobExecutor) HandleSuccessJobs(successfulJobs []models.JobModel) {
	for i, successfulJob := range successfulJobs {
		schedule, parseErr := cron.Parse(successfulJob.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(successfulJob.LastExecutionDate)
		jobExecutor.logger.Println(fmt.Sprintf("executed job %v", successfulJob.ID))
		successfulJobs[i].LastExecutionDate = executionTime
		jobExecutor.AddNewProcess(successfulJobs[i], schedule.Next(executionTime))
	}
}

func (jobExecutor *JobExecutor) HandleFailedJobs(erroredJobs []models.JobModel) {
	for i, erroredJob := range erroredJobs {
		schedule, parseErr := cron.Parse(erroredJob.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(erroredJob.LastExecutionDate)
		jobExecutor.logger.Println(fmt.Sprintf("executed job %v", erroredJob.ID))
		erroredJobs[i].LastExecutionDate = executionTime
		jobExecutor.AddNewProcess(erroredJobs[i], schedule.Next(executionTime))
	}
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
		if len(line) < 1 {
			continue
		}
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

func (jobExecutor *JobExecutor) AddNewProcess(job models.JobModel, executeTime time.Time) {
	go func(j models.JobModel, e time.Time) {
		execTime := time.Now().UTC().Add(e.Sub(j.LastExecutionDate))
		ticker := time.NewTicker(time.Duration(1) * time.Second)
		for {
			select {
			case <-ticker.C:
				if time.Now().UTC().After(execTime) {
					jobExecutor.PendingJobs <- j
					return
				}
			case <-jobExecutor.context.Done():
				jobExecutor.logger.Println("stopping scheduled job with id", j.ID)
				return
			}
		}
	}(job, executeTime)
}
