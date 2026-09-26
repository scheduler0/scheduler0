package node

import (
	"context"
	"fmt"
	"net"
	"scheduler0/pkg/config"
	"scheduler0/pkg/models"
	"scheduler0/pkg/network"
	secrets "scheduler0/pkg/secrets"
	async_task_service "scheduler0/pkg/service/async_task"
	"scheduler0/pkg/utils"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/segmentio/ksuid"
)

type tcpServer struct {
	context           context.Context
	logger            hclog.Logger
	ln                network.Listener
	nodeService       NodeService
	asyncTaskService  async_task_service.AsyncTaskService
	scheduler0Configs config.Scheduler0Config
	scheduler0Secrets secrets.Scheduler0Secrets
	metrics           *utils.TCPServerMetrics
}

func NewTCPServer(ctx context.Context, logger hclog.Logger, ln network.Listener, scheduler0Configs config.Scheduler0Config, scheduler0Secrets secrets.Scheduler0Secrets, nodeService NodeService, asyncTaskService async_task_service.AsyncTaskService) Server {
	metrics := utils.NewTCPServerMetrics()
	namedLogger := logger.Named("node-tcp-server")

	// Start periodic metrics logging
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				snapshot := metrics.GetMetrics()
				if snapshot.Total > 0 {
					namedLogger.Info("TCP server metrics summary",
						"totalRequests", snapshot.Total,
						"successCount", snapshot.Success,
						"connectionResetCount", snapshot.ConnectionReset,
						"otherErrorsCount", snapshot.OtherErrors,
						"successRate", fmt.Sprintf("%.2f%%", snapshot.SuccessRate),
						"timeoutRate", fmt.Sprintf("%.2f%%", snapshot.TimeoutRate),
						"errorRate", fmt.Sprintf("%.2f%%", snapshot.ErrorRate),
						"waitDurationAvg", snapshot.WaitDurationAvg,
						"waitDurationMin", snapshot.WaitDurationMin,
						"waitDurationMax", snapshot.WaitDurationMax,
						"writeDurationAvg", snapshot.WriteDurationAvg,
						"writeDurationMin", snapshot.WriteDurationMin,
						"writeDurationMax", snapshot.WriteDurationMax,
						"totalDurationAvg", snapshot.TotalDurationAvg,
						"totalDurationMin", snapshot.TotalDurationMin,
						"totalDurationMax", snapshot.TotalDurationMax,
					)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return &tcpServer{
		context:           ctx,
		logger:            namedLogger,
		ln:                ln,
		nodeService:       nodeService,
		asyncTaskService:  asyncTaskService,
		scheduler0Configs: scheduler0Configs,
		scheduler0Secrets: scheduler0Secrets,
		metrics:           metrics,
	}
}

func (server *tcpServer) SetupTCPListener() {
	server.logger.Info("starting TCP listener for node-to-node communication")
	for {
		conn, acceptErr := server.ln.Accept()
		if acceptErr != nil {
			server.logger.Error("failed to accept connection", "error", acceptErr)
			return
		}

		remoteAddr := conn.RemoteAddr().String()
		server.logger.Debug("accepted new connection", "remoteAddress", remoteAddr)

		go func(c net.Conn) {
			defer func() {
				conn.Close()
				server.logger.Debug("connection closed", "remoteAddress", remoteAddr)
			}()

			connectionContextWithCancel, calFunc := context.WithCancel(server.context)
			defer func() {
				calFunc()
			}()

			payload, decodeErr := decode(conn)
			if decodeErr != nil {
				server.logger.Error("failed to decode incoming connection", "error", decodeErr, "remoteAddress", remoteAddr)
				return
			}

			server.logger.Debug("decoded payload", "payloadType", fmt.Sprintf("%T", payload), "remoteAddress", remoteAddr)

			authPayload, ok := payload.(*models.NodeAuth)
			if ok {
				server.logger.Debug("received node auth request", "remoteAddress", remoteAddr)
				connected := server.HandleNodeAuthRequest(*authPayload)
				_, writeError := connected.WriteTo(c)
				if writeError != nil {
					server.logger.Error("failed to write node auth response", "error", writeError, "remoteAddress", remoteAddr)
				} else {
					server.logger.Debug("sent node auth response", "response", string(connected), "remoteAddress", remoteAddr)
				}
				return
			}

			fetchRemoteDataPayload, ok := payload.(*models.FetchRemoteData)
			if ok {
				server.logger.Debug("received fetch remote data request", "requestId", fetchRemoteDataPayload.RequestId, "remoteAddress", remoteAddr)

				// Handle special commands
				if fetchRemoteDataPayload.RequestId == "stop_jobs" {
					server.logger.Info("received stop jobs command", "remoteAddress", remoteAddr)
					response := server.HandleStopJobsCommand(*fetchRemoteDataPayload)
					_, writeError := response.WriteTo(conn)
					if writeError != nil {
						server.logger.Error("failed to write stop jobs response", "error", writeError, "remoteAddress", remoteAddr)
					} else {
						server.logger.Info("sent stop jobs response", "response", string(response), "remoteAddress", remoteAddr)
					}
					return
				}

				if fetchRemoteDataPayload.RequestId == "start_jobs" {
					server.logger.Info("received start jobs command", "remoteAddress", remoteAddr)
					response := server.HandleStartJobsCommand(*fetchRemoteDataPayload)
					_, writeError := response.WriteTo(conn)
					if writeError != nil {
						server.logger.Error("failed to write start jobs response", "error", writeError, "remoteAddress", remoteAddr)
					} else {
						server.logger.Info("sent start jobs response", "response", string(response), "remoteAddress", remoteAddr)
					}
					return
				}

				if fetchRemoteDataPayload.RequestId == "" {
					server.logger.Debug("beginning uncommitted logs fetch request (phase 1)", "remoteAddress", remoteAddr)
					requestId := server.BeginUncommittedLogsFetchRequest(*fetchRemoteDataPayload)
					_, writeError := requestId.WriteTo(conn)
					if writeError != nil {
						server.logger.Error("failed to write uncommitted logs fetch request id", "error", writeError, "remoteAddress", remoteAddr)
					} else {
						server.logger.Info("sent uncommitted logs fetch request id", "requestId", string(requestId), "remoteAddress", remoteAddr)
					}
					return
				} else {
					requestStartTime := time.Now()
					server.logger.Debug("handling uncommitted logs fetch request (phase 2)", "requestId", fetchRemoteDataPayload.RequestId, "remoteAddress", remoteAddr)

					// Record total request
					server.metrics.RecordTotal()

					asyncTask := server.HandelUncommittedLogsFetchRequest(connectionContextWithCancel, *fetchRemoteDataPayload)

					// Record write operation
					writeStartTime := time.Now()
					_, writeError := asyncTask.WriteTo(conn)
					writeDuration := time.Since(writeStartTime)
					totalDuration := time.Since(requestStartTime)

					// Record metrics
					server.metrics.RecordWriteDuration(writeDuration)
					server.metrics.RecordTotalDuration(totalDuration)

					if writeError != nil {
						server.metrics.RecordError(writeError)
						server.logger.Error("failed to write uncommitted logs fetch response", "error", writeError, "requestId", fetchRemoteDataPayload.RequestId, "remoteAddress", remoteAddr, "writeDuration", writeDuration, "totalDuration", totalDuration)
					} else {
						server.metrics.RecordSuccess()
						server.logger.Info("sent uncommitted logs fetch response", "requestId", fetchRemoteDataPayload.RequestId, "taskState", asyncTask.State, "remoteAddress", remoteAddr, "writeDuration", writeDuration, "totalDuration", totalDuration)
					}
					return
				}
			}

			quotaAllocationPayload, ok := payload.(*models.QuotaAllocation)
			if ok {
				server.logger.Debug("received quota allocation request", "remoteAddress", remoteAddr, "accountCount", len(quotaAllocationPayload.AccountAllocations))
				response := server.HandleQuotaAllocation(*quotaAllocationPayload)
				_, writeError := response.WriteTo(conn)
				if writeError != nil {
					server.logger.Error("failed to write quota allocation response", "error", writeError, "remoteAddress", remoteAddr)
				} else {
					server.logger.Info("sent quota allocation response", "response", string(response), "remoteAddress", remoteAddr)
				}
				return
			}

			localQuotaRequestPayload, ok := payload.(*models.LocalQuotaRequest)
			if ok {
				server.logger.Debug("received local quota request", "remoteAddress", remoteAddr)
				response := server.HandleLocalQuotaRequest(*localQuotaRequestPayload)
				_, writeError := response.WriteTo(conn)
				if writeError != nil {
					server.logger.Error("failed to write local quota response", "error", writeError, "remoteAddress", remoteAddr)
				} else {
					server.logger.Info("sent local quota response", "remoteAddress", remoteAddr, "accountCount", len(response.AccountAllocations))
				}
				return
			}

			accountExhaustionPayload, ok := payload.(*models.AccountExhaustion)
			if ok {
				server.logger.Debug("received account exhaustion notification", "remoteAddress", remoteAddr, "accountId", accountExhaustionPayload.AccountId)
				response := server.HandleAccountExhaustion(*accountExhaustionPayload)
				_, writeError := response.WriteTo(conn)
				if writeError != nil {
					server.logger.Error("failed to write account exhaustion response", "error", writeError, "remoteAddress", remoteAddr)
				} else {
					server.logger.Info("sent account exhaustion response", "response", string(response), "remoteAddress", remoteAddr, "accountId", accountExhaustionPayload.AccountId)
				}
				return
			}

			jobUpdateRequestPayload, ok := payload.(*models.JobUpdateRequest)
			if ok {
				server.logger.Debug("received job update request", "remoteAddress", remoteAddr, "jobId", jobUpdateRequestPayload.Job.ID)
				response := server.HandleJobUpdateRequest(*jobUpdateRequestPayload)
				_, writeError := response.WriteTo(conn)
				if writeError != nil {
					server.logger.Error("failed to write job update response", "error", writeError, "remoteAddress", remoteAddr)
				} else {
					server.logger.Info("sent job update response", "response", string(response), "remoteAddress", remoteAddr, "jobId", jobUpdateRequestPayload.Job.ID)
				}
				return
			}

			server.logger.Warn("unexpected payload type received", "payloadType", fmt.Sprintf("%T", payload), "remoteAddress", remoteAddr)
		}(conn)
	}
}

func (server *tcpServer) HandleNodeAuthRequest(nodeAuthPayload models.NodeAuth) models.String {
	scheduler0Secrets := server.scheduler0Secrets.GetSecrets()
	if nodeAuthPayload.AuthUsername != scheduler0Secrets.AuthUsername || nodeAuthPayload.AuthPassword != scheduler0Secrets.AuthPassword {
		server.logger.Warn("node auth request failed: invalid credentials", "username", nodeAuthPayload.AuthUsername)
		return models.String("incorrect_credentials")
	}
	server.logger.Debug("node auth request successful", "username", nodeAuthPayload.AuthUsername)
	return models.String("connected")
}

func (server *tcpServer) BeginUncommittedLogsFetchRequest(payload models.FetchRemoteData) models.String {
	scheduler0Secrets := server.scheduler0Secrets.GetSecrets()

	if payload.AuthUsername != scheduler0Secrets.AuthUsername || payload.AuthPassword != scheduler0Secrets.AuthPassword {
		server.logger.Warn("uncommitted logs fetch request failed: invalid credentials", "username", payload.AuthUsername)
		return models.String("incorrect_credentials")
	}

	requestId := ksuid.New().String()
	server.logger.Info("beginning uncommitted logs fetch request", "requestId", requestId, "username", payload.AuthUsername)
	server.nodeService.ReturnUncommittedLogs(requestId)
	server.logger.Debug("uncommitted logs fetch request initiated", "requestId", requestId)
	return models.String(requestId)
}

func (server *tcpServer) HandleStopJobsCommand(payload models.FetchRemoteData) models.String {
	scheduler0Secrets := server.scheduler0Secrets.GetSecrets()

	if payload.AuthPassword != scheduler0Secrets.AuthPassword || payload.AuthUsername != scheduler0Secrets.AuthUsername {
		server.logger.Error("invalid credentials for stop jobs command")
		return models.String("incorrect_credentials")
	}

	server.logger.Info("received stop jobs command, clearing schedule queue")
	server.nodeService.StopJobs()
	return models.String("stopped")
}

func (server *tcpServer) HandleStartJobsCommand(payload models.FetchRemoteData) models.String {
	scheduler0Secrets := server.scheduler0Secrets.GetSecrets()

	if payload.AuthPassword != scheduler0Secrets.AuthPassword || payload.AuthUsername != scheduler0Secrets.AuthUsername {
		server.logger.Error("invalid credentials for start jobs command")
		return models.String("incorrect_credentials")
	}

	server.logger.Info("received start jobs command, recovering jobs")
	server.nodeService.StartJobs()
	return models.String("started")
}

func (server *tcpServer) HandleQuotaAllocation(payload models.QuotaAllocation) models.String {
	scheduler0Secrets := server.scheduler0Secrets.GetSecrets()

	if payload.AuthUsername != scheduler0Secrets.AuthUsername || payload.AuthPassword != scheduler0Secrets.AuthPassword {
		server.logger.Warn("quota allocation request failed: invalid credentials", "username", payload.AuthUsername)
		return models.String("incorrect_credentials")
	}

	server.logger.Info("received quota allocation", "accountCount", len(payload.AccountAllocations), "allocations", payload.AccountAllocations)

	// Store quota allocations in the executor service
	// The executor will use these local quotas when executing jobs
	if server.nodeService == nil {
		server.logger.Error("node service not available, cannot store quota allocations")
		return models.String("error: node service not available")
	}

	// Forward quota allocation to executor via node service
	server.logger.Debug("forwarding quota allocation to executor", "accountCount", len(payload.AccountAllocations))
	if err := server.nodeService.UpdateLocalQuotaAllocations(payload.AccountAllocations); err != nil {
		server.logger.Error("failed to update quota allocations", "error", err)
		return models.String("error: failed to update quota allocations")
	}

	return models.String("quota_allocation_received")
}

func (server *tcpServer) HandleLocalQuotaRequest(payload models.LocalQuotaRequest) models.LocalQuotaResponse {
	scheduler0Secrets := server.scheduler0Secrets.GetSecrets()

	if payload.AuthUsername != scheduler0Secrets.AuthUsername || payload.AuthPassword != scheduler0Secrets.AuthPassword {
		server.logger.Warn("local quota request failed: invalid credentials", "username", payload.AuthUsername)
		return models.LocalQuotaResponse{
			AccountAllocations: make(map[uint64]uint64),
		}
	}

	server.logger.Debug("received local quota request")

	// Get local quota allocations from the executor service
	if server.nodeService == nil {
		server.logger.Error("node service not available, cannot get local quota allocations")
		return models.LocalQuotaResponse{
			AccountAllocations: make(map[uint64]uint64),
		}
	}

	// Get local quota allocations from executor via node service
	localQuotas := server.nodeService.GetLocalQuotaAllocations()
	server.logger.Debug("retrieved local quota allocations", "accountCount", len(localQuotas))

	return models.LocalQuotaResponse{
		AccountAllocations: localQuotas,
	}
}

func (server *tcpServer) HandleAccountExhaustion(payload models.AccountExhaustion) models.String {
	scheduler0Secrets := server.scheduler0Secrets.GetSecrets()

	if payload.AuthUsername != scheduler0Secrets.AuthUsername || payload.AuthPassword != scheduler0Secrets.AuthPassword {
		server.logger.Warn("account exhaustion notification failed: invalid credentials", "username", payload.AuthUsername)
		return models.String("incorrect_credentials")
	}

	server.logger.Info("received account exhaustion notification", "accountId", payload.AccountId)

	// Update all jobs for this account to inactive
	if server.nodeService == nil {
		server.logger.Error("node service not available, cannot update job status")
		return models.String("error: node service not available")
	}

	// Update all jobs for this account to inactive via node service
	err := server.nodeService.UpdateJobsStatusByAccountId(payload.AccountId, models.JobStatusInactive)
	if err != nil {
		server.logger.Error("failed to update jobs status to inactive", "accountId", payload.AccountId, "error", err)
		return models.String(fmt.Sprintf("error: failed to update jobs status: %s", err.Error()))
	}

	server.logger.Info("successfully updated jobs to inactive for exhausted account", "accountId", payload.AccountId)
	return models.String("account_exhaustion_processed")
}

func (server *tcpServer) HandleJobUpdateRequest(payload models.JobUpdateRequest) models.String {
	scheduler0Secrets := server.scheduler0Secrets.GetSecrets()

	if payload.AuthUsername != scheduler0Secrets.AuthUsername || payload.AuthPassword != scheduler0Secrets.AuthPassword {
		server.logger.Warn("job update request failed: invalid credentials", "username", payload.AuthUsername, "jobId", payload.Job.ID)
		return models.String("incorrect_credentials")
	}

	server.logger.Info("received job update request", "jobId", payload.Job.ID, "status", payload.Job.Status)

	// Update the job via node service
	if server.nodeService == nil {
		server.logger.Error("node service not available, cannot update job")
		return models.String("error: node service not available")
	}

	err := server.nodeService.UpdateJobOnLeader(payload.Job)
	if err != nil {
		server.logger.Error("failed to update job", "jobId", payload.Job.ID, "error", err)
		return models.String(fmt.Sprintf("error: failed to update job: %s", err.Error()))
	}

	server.logger.Info("successfully updated job", "jobId", payload.Job.ID, "status", payload.Job.Status)
	return models.String("job_updated")
}

func (server *tcpServer) HandelUncommittedLogsFetchRequest(ctx context.Context, payload models.FetchRemoteData) models.AsyncTask {
	waitStartTime := time.Now()
	scheduler0Secrets := server.scheduler0Secrets.GetSecrets()

	if payload.AuthUsername != scheduler0Secrets.AuthUsername || payload.AuthPassword != scheduler0Secrets.AuthPassword {
		server.logger.Error("uncommitted logs fetch request failed: invalid credentials", "requestId", payload.RequestId, "username", payload.AuthUsername)
		return models.AsyncTask{}
	}

	server.logger.Debug("fetching uncommitted logs async task", "requestId", payload.RequestId)

	task, err := server.asyncTaskService.GetTaskWithRequestIdNonBlocking(payload.RequestId, 1)
	if err != nil {
		server.logger.Error("could not find async task (non-blocking)", "requestId", payload.RequestId, "error", err.Message)
		return models.AsyncTask{}
	}

	if task.State == models.AsyncTaskSuccess {
		waitDuration := time.Since(waitStartTime)
		server.metrics.RecordWaitDuration(waitDuration)
		server.logger.Info("async task already completed (non-blocking)", "requestId", payload.RequestId, "taskState", task.State, "waitDuration", waitDuration)
		return *task
	}

	server.logger.Debug("async task not ready, waiting (blocking)", "requestId", payload.RequestId, "currentState", task.State)

	taskCh, subId, err := server.asyncTaskService.GetTaskWithRequestIdBlocking(payload.RequestId, 1)
	if err != nil {
		server.logger.Error("could not get async task (blocking)", "requestId", payload.RequestId, "error", err.Message)
		return models.AsyncTask{}
	}

	server.logger.Debug("subscribed to async task updates", "requestId", payload.RequestId, "subscriberId", subId)

	for {
		select {
		case task := <-taskCh:
			waitDuration := time.Since(waitStartTime)
			server.metrics.RecordWaitDuration(waitDuration)
			server.logger.Info("async task completed, returning result", "requestId", payload.RequestId, "taskState", task.State, "waitDuration", waitDuration)
			return task
		case <-ctx.Done():
			waitDuration := time.Since(waitStartTime)
			server.metrics.RecordWaitDuration(waitDuration)
			server.logger.Warn("context cancelled while waiting for async task", "requestId", payload.RequestId, "waitDuration", waitDuration)
			taskIdInt, err := server.asyncTaskService.GetTaskIdWithRequestId(payload.RequestId, 1)
			if err != nil {
				server.logger.Error("failed to get task id for subscriber cleanup", "requestId", payload.RequestId, "error", err.Message)
				return models.AsyncTask{}
			}
			delErr := server.asyncTaskService.DeleteSubscriber(taskIdInt, subId)
			if taskCh != nil {
				close(taskCh)
				server.logger.Debug("closed task channel", "requestId", payload.RequestId, "taskId", taskIdInt, "subscriberId", subId)
			} else {
				server.logger.Warn("attempted to close nil task channel", "taskId", taskIdInt, "requestId", payload.RequestId)
			}
			if delErr != nil {
				server.logger.Error("failed to delete subscriber", "taskId", taskIdInt, "subscriberId", subId, "requestId", payload.RequestId, "error", delErr.Error())
				return models.AsyncTask{}
			}
			server.logger.Debug("subscriber deleted successfully", "taskId", taskIdInt, "subscriberId", subId, "requestId", payload.RequestId)
			return models.AsyncTask{}
		}
	}

}
