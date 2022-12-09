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
	context               context.Context
	Raft                  *raft.Raft
	pendingExecutions     chan models.JobModel
	pendingInvocationJobs []models.JobModel
	jobRepo               repository.Job
	logger                *log.Logger
	cancelReq             context.CancelFunc
	executor              *executors.Service
	threshold             time.Time
	ticker                *time.Ticker
	mtx                   sync.Mutex
	invocationLock        sync.Mutex
	once                  sync.Once
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
	logFilePath := fmt.Sprintf("%v/%v", dirPath, constants.ExecutionLogsCommitFile)
	if os.IsNotExist(err) {
		_, createErr := os.Create(logFilePath)
		if createErr != nil {
			logger.Fatalln(fmt.Errorf("failed to create the %s file %s \n", logFilePath, err))
		}
	}
	logFilePath = fmt.Sprintf("%v/%v", dirPath, constants.ExecutionLogsUnCommitFile)
	if os.IsNotExist(err) {
		_, createErr := os.Create(logFilePath)
		if createErr != nil {
			logger.Fatalln(fmt.Errorf("failed to create the %s file %s \n", logFilePath, err))
		}
	}
	if err != nil {
		logger.Fatalln("logs file close error")
	}
	return &JobExecutor{
		pendingExecutions: make(chan models.JobModel, constants.JobMaxBatchSize),
		jobRepo:           jobRepository,
		logger:            logger,
		context:           ctx,
		cancelReq:         cancel,
		executor:          executors.NewService(logger),
	}
}

// invokeJob executes and http job
func (jobExecutor *JobExecutor) invokeJob(pendingJob models.JobModel) {
	configs := config.GetConfigurations(jobExecutor.logger)
	jobExecutor.threshold = time.Now().Add(time.Duration(configs.JobInvocationDebounceDelay) * time.Millisecond)

	jobExecutor.pendingInvocationJobs = append(jobExecutor.pendingInvocationJobs, pendingJob)

	jobExecutor.once.Do(func() {
		go func() {
			defer func() {
				jobExecutor.invocationLock.Lock()
				jobExecutor.ticker.Stop()
				jobExecutor.once = sync.Once{}
				jobExecutor.invocationLock.Unlock()
			}()

			jobExecutor.ticker = time.NewTicker(time.Duration(500) * time.Millisecond)

			for {
				select {
				case <-jobExecutor.ticker.C:
					jobExecutor.invocationLock.Lock()
					if time.Now().After(jobExecutor.threshold) {
						jobIDs := make([]int64, 0)
						pendingJobs := jobExecutor.pendingInvocationJobs
						for _, pendingJobInvocation := range pendingJobs {
							jobIDs = append(jobIDs, pendingJobInvocation.ID)
						}

						jobs, batchGetError := jobExecutor.jobRepo.BatchGetJobsByID(jobIDs)
						if batchGetError != nil {
							jobExecutor.logger.Println(fmt.Sprintf("batch query error:: %s", batchGetError.Message))
							return
						}

						getPendingJob := func(jobID int64) *models.JobModel {
							for _, pendingJobInvocation := range pendingJobs {
								if pendingJobInvocation.ID == jobID {
									return &pendingJobInvocation
								}
							}
							return nil
						}

						jobExecutor.logger.Println(fmt.Sprintf("batched queried %v", len(jobs)))

						jobsToExecute := make([]models.JobModel, 0)

						for _, job := range jobs {
							pendingJobInvocation := getPendingJob(job.ID)
							if pendingJobInvocation != nil {
								jobsToExecute = append(jobsToExecute, *pendingJobInvocation)
							}
						}

						// TODO: execute job based on job execution type
						if len(jobsToExecute) > 0 {
							jobExecutor.executor.ExecuteHTTP(
								jobsToExecute,
								jobExecutor.context,
								jobExecutor.handleSuccessJobs,
								jobExecutor.handleFailedJobs,
							)
						}
						jobExecutor.invocationLock.Unlock()
						return
					}
					jobExecutor.invocationLock.Unlock()
				case <-jobExecutor.context.Done():
					return
				}
			}
		}()
	})
}

// ListenOnInvocationChannels periodically checks channels for updates
func (jobExecutor *JobExecutor) ListenOnInvocationChannels() {
	wg := sync.WaitGroup{}
	for i := 0; i < constants.JobMaxBatchSize; i++ {
		wg.Add(i)
		go func(wg *sync.WaitGroup) {
			for {
				select {
				case pendingJob := <-jobExecutor.pendingExecutions:
					jobExecutor.invokeJob(pendingJob)
					jobExecutor.logger.Println(fmt.Sprintf("job %v pending execution", pendingJob.ID))
				case <-jobExecutor.context.Done():
					wg.Done()
				}
			}
		}(&wg)
	}
	wg.Wait()
}

func (jobExecutor *JobExecutor) LogJobExecutionStateOnLeader(pendingJobs []models.JobModel, actionType constants.Command) {
	configs := config.GetConfigurations(jobExecutor.logger)
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

func (jobExecutor *JobExecutor) QueueExecutions(jobQueueParams []interface{}) {
	configs := config.GetConfigurations(jobExecutor.logger)

	jobExecutor.QueueLogs(jobQueueParams)

	if jobQueueParams[0].(string) != configs.RaftAddress {
		return
	}

	lowerBound := jobQueueParams[1].(int64)
	upperBound := jobQueueParams[2].(int64)
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

	js := []models.JobModel{}

	for i, job := range jobs {
		scheduleJobState, prepareFound := jobExecutor.getJobLastStateLogIgnoringQueueState(constants.ScheduledExecutionLogPrefix, job.ID)
		successJobState, commitFound := jobExecutor.getJobLastStateLogIgnoringQueueState(constants.SuccessfulExecutionLogPrefix, job.ID)
		failedJobState, errorFound := jobExecutor.getJobLastStateLogIgnoringQueueState(constants.FailedExecutionLogPrefix, job.ID)

		if !prepareFound && !commitFound && !errorFound {
			executionUUID, err := uuid.GenerateUUID()
			if err != nil {
				jobExecutor.logger.Fatalln(fmt.Sprintf("failed to created execution id for job %s", err.Error()))
			}

			jobs[i].LastExecutionDate = job.DateCreated
			jobs[i].ExecutionId = executionUUID
		}

		if prepareFound && !commitFound && !errorFound {
			jobs[i].LastExecutionDate = scheduleJobState.Data[0].LastExecutionDate
			jobs[i].ExecutionId = scheduleJobState.Data[0].ExecutionId
		}

		if prepareFound && commitFound && !errorFound {
			if scheduleJobState.ExecutionTime.After(successJobState.ExecutionTime) {
				jobs[i].LastExecutionDate = scheduleJobState.Data[0].LastExecutionDate
				jobs[i].ExecutionId = scheduleJobState.Data[0].ExecutionId
			} else {
				executionUUID, err := uuid.GenerateUUID()
				if err != nil {
					jobExecutor.logger.Fatalln(fmt.Sprintf("failed to created execution id for job %s", err.Error()))
				}
				jobs[i].LastExecutionDate = successJobState.ExecutionTime
				jobs[i].ExecutionId = executionUUID
			}
		}

		if prepareFound && commitFound && errorFound {
			if scheduleJobState.ExecutionTime.After(successJobState.ExecutionTime) && scheduleJobState.ExecutionTime.After(failedJobState.ExecutionTime) {
				jobs[i].LastExecutionDate = scheduleJobState.Data[0].LastExecutionDate
				jobs[i].ExecutionId = scheduleJobState.Data[0].ExecutionId
			} else {
				executionUUID, err := uuid.GenerateUUID()
				if err != nil {
					jobExecutor.logger.Fatalln(fmt.Sprintf("failed to created execution id for job %s", err.Error()))
				}

				jobs[i].LastExecutionDate = job.DateCreated
				jobs[i].ExecutionId = executionUUID
			}
		}

		schedule, parseErr := cron.Parse(jobs[i].Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(jobs[i].LastExecutionDate)
		jobExecutor.AddNewProcess(jobs[i], executionTime)
		js = append(js, jobs[i])
	}
	configs := config.GetConfigurations(jobExecutor.logger)

	jobExecutor.LogJobScheduledExecutions(models.JobStateLog{
		ServerAddress: fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port),
		State:         constants.CommandTypeScheduleJobExecutions,
		Data:          js,
	}, false)
	jobExecutor.logger.Println("scheduled job", jobs[0].ID, "to", jobs[len(jobs)-1].ID)
}

func (jobExecutor *JobExecutor) QueueLogs(jobQueueParams []interface{}) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()
	entry := fmt.Sprintf("queue, %v, %v, %v, %v\n", jobQueueParams[0], jobQueueParams[1], jobQueueParams[2], time.Now().UTC())
	jobExecutor.writeJobExecutionLog(entry, true)
}

func (jobExecutor *JobExecutor) LogJobScheduledExecutions(jobState models.JobStateLog, fromLeader bool) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()
	entry := ""
	for _, job := range jobState.Data {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(job.LastExecutionDate)
		entry += fmt.Sprintf("%v, %v, %v, %v, %v, %v\n", constants.ScheduledExecutionLogPrefix, jobState.ServerAddress, job.ID, job.ExecutionId, job.LastExecutionDate.String(), executionTime.String())
	}
	jobExecutor.writeJobExecutionLog(entry, fromLeader)
}

func (jobExecutor *JobExecutor) LogSuccessfulJobExecutions(jobState models.JobStateLog, fromLeader bool) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()
	entry := ""
	for _, job := range jobState.Data {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(job.LastExecutionDate)
		entry += fmt.Sprintf("%v, %v, %v, %v, %v, %v\n", constants.SuccessfulExecutionLogPrefix, jobState.ServerAddress, job.ID, job.ExecutionId, job.LastExecutionDate.String(), executionTime.String())
	}
	jobExecutor.writeJobExecutionLog(entry, fromLeader)
}

func (jobExecutor *JobExecutor) LogFailedJobExecutions(jobState models.JobStateLog, fromLeader bool) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()
	entry := ""
	for _, job := range jobState.Data {
		schedule, parseErr := cron.Parse(job.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(job.LastExecutionDate)
		entry += fmt.Sprintf("%v, %v, %v, %v, %v, %v\n", constants.FailedExecutionLogPrefix, jobState.ServerAddress, job.ID, job.ExecutionId, job.LastExecutionDate.String(), executionTime.String())
	}
	jobExecutor.writeJobExecutionLog(entry, fromLeader)
}

func (jobExecutor *JobExecutor) StopAll() {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	jobExecutor.logger.Println("stopped all scheduled job")
	jobExecutor.pendingExecutions = make(chan models.JobModel)
	jobExecutor.cancelReq()
	ctx, cancel := context.WithCancel(context.Background())
	jobExecutor.cancelReq = cancel
	jobExecutor.context = ctx
}

func (jobExecutor *JobExecutor) handleSuccessJobs(successfulJobs []models.JobModel) {
	for i, successfulJob := range successfulJobs {
		schedule, parseErr := cron.Parse(successfulJob.Spec)
		if parseErr != nil {
			jobExecutor.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
		}
		executionTime := schedule.Next(successfulJob.LastExecutionDate)
		jobExecutor.logger.Println(fmt.Sprintf("executed job %v", successfulJob.ID))
		successfulJobs[i].LastExecutionDate = executionTime
	}
	configs := config.GetConfigurations(jobExecutor.logger)
	jobExecutor.LogSuccessfulJobExecutions(models.JobStateLog{
		ServerAddress: fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port),
		State:         constants.CommandTypeSuccessfulJobExecutions,
		Data:          successfulJobs,
	}, false)
	jobExecutor.Schedule(successfulJobs)
}

func (jobExecutor *JobExecutor) handleFailedJobs(erroredJobs []models.JobModel) {
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
	configs := config.GetConfigurations(jobExecutor.logger)
	jobExecutor.LogFailedJobExecutions(models.JobStateLog{
		ServerAddress: fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port),
		State:         constants.CommandTypeFailedJobExecutions,
		Data:          erroredJobs,
	}, false)
	// TODO: Reschedule failed jobs
}

func (jobExecutor *JobExecutor) writeJobExecutionLog(entry string, commit bool) {
	dir, err := os.Getwd()
	if err != nil {
		jobExecutor.logger.Fatalln(fmt.Errorf("fatal error getting working dir: %s \n", err))
	}
	dirPath := fmt.Sprintf("%v/%v", dir, constants.ExecutionLogsDir)
	fileName := constants.ExecutionLogsUnCommitFile

	if commit {
		fileName = constants.ExecutionLogsUnCommitFile
	}

	logFilePath := fmt.Sprintf("%v/%v", dirPath, fileName)
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		jobExecutor.logger.Println("open lof file error", err)
	}
	defer func(logFile *os.File) {
		err := logFile.Close()
		if err != nil {
			log.Fatalln("execution log write error::", err)
		}
	}(logFile)

	_, writeErr := logFile.WriteString(entry)
	if writeErr != nil {
		log.Fatalln("execution log write error::", writeErr)
	}
}

func (jobExecutor *JobExecutor) getJobLastStateLogIgnoringQueueState(state string, jobId int64) (models.JobStateLog, bool) {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	dir, err := os.Getwd()
	if err != nil {
		jobExecutor.logger.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}

	lines := []string{}

	dirPath := fmt.Sprintf("%v/%v", dir, constants.ExecutionLogsDir)
	uncommittedLogfilePath := fmt.Sprintf("%v/%v", dirPath, constants.ExecutionLogsUnCommitFile)
	committedLogfilePath := fmt.Sprintf("%v/%v", dirPath, constants.ExecutionLogsCommitFile)
	logFile, err := os.OpenFile(uncommittedLogfilePath, os.O_RDONLY, os.ModePerm)
	data, err := io.ReadAll(logFile)
	fileData := string(data)
	if err == nil {
		lines = strings.Split(fileData, "\n")
	}
	logFile.Close()
	logFile, err = os.OpenFile(committedLogfilePath, os.O_RDONLY, os.ModePerm)
	data, err = io.ReadAll(logFile)
	fileData = string(data)
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
		if commandToken == constants.QueueExecutionLogPrefix {
			continue
		}
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

			if executionTime.After(lastJobStatLog.ExecutionTime) || lastJobStatLog.ExecutionTime.IsZero() {
				lastJobStatLog.ExecutionTime = executionTime
				lastJobStatLog.ServerAddress = serverAddressToken
				lastJobStatLog.Data[0].LastExecutionDate = lastExecutionTime
				lastJobStatLog.Data[0].ExecutionId = executionIdToken
				found = true
			}
		}
	}

	switch state {
	case constants.ScheduledExecutionLogPrefix:
		lastJobStatLog.State = constants.CommandTypeScheduleJobExecutions
		break
	case constants.SuccessfulExecutionLogPrefix:
		lastJobStatLog.State = constants.CommandTypeSuccessfulJobExecutions
		break
	case constants.FailedExecutionLogPrefix:
		lastJobStatLog.State = constants.CommandTypeFailedJobExecutions
		break
	}

	return lastJobStatLog, found
}

func (jobExecutor *JobExecutor) GetJobLogsForServer(serverAddress string) map[int]*models.JobStateLog {
	jobExecutor.mtx.Lock()
	defer jobExecutor.mtx.Unlock()

	dir, err := os.Getwd()
	if err != nil {
		jobExecutor.logger.Fatalln(fmt.Errorf("Fatal error getting working dir: %s \n", err))
	}

	lines := []string{}

	dirPath := fmt.Sprintf("%v/%v", dir, constants.ExecutionLogsDir)
	uncommittedLogfilePath := fmt.Sprintf("%v/%v", dirPath, constants.ExecutionLogsUnCommitFile)
	committedLogfilePath := fmt.Sprintf("%v/%v", dirPath, constants.ExecutionLogsCommitFile)
	logFile, err := os.OpenFile(uncommittedLogfilePath, os.O_RDONLY, os.ModePerm)
	data, err := io.ReadAll(logFile)
	fileData := string(data)
	if err == nil {
		lines = strings.Split(fileData, "\n")
	}
	logFile.Close()
	logFile, err = os.OpenFile(committedLogfilePath, os.O_RDONLY, os.ModePerm)
	data, err = io.ReadAll(logFile)
	fileData = string(data)
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
		if stateToken == constants.QueueExecutionLogPrefix {
			continue
		}
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
			case constants.ScheduledExecutionLogPrefix:
				jobState.State = constants.CommandTypeScheduleJobExecutions
				break
			case constants.SuccessfulExecutionLogPrefix:
				jobState.State = constants.CommandTypeSuccessfulJobExecutions
				break
			case constants.FailedExecutionLogPrefix:
				jobState.State = constants.CommandTypeFailedJobExecutions
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
					jobExecutor.pendingExecutions <- j
					return
				}
			case <-jobExecutor.context.Done():
				jobExecutor.logger.Println("stopping scheduled job with id", j.ID)
				return
			}
		}
	}(job, executeTime)
}
