package node

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"scheduler0/pkg/alerts"
	"scheduler0/pkg/config"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/network"
	"scheduler0/pkg/repository/async_task"
	"scheduler0/pkg/repository/job"
	"scheduler0/pkg/repository/job_execution"
	"scheduler0/pkg/repository/job_queue"
	"scheduler0/pkg/repository/project"
	"scheduler0/pkg/secrets"
	async_task_service "scheduler0/pkg/service/async_task"
	"scheduler0/pkg/service/etcd"
	"scheduler0/pkg/service/executor"
	"scheduler0/pkg/service/processor"
	"scheduler0/pkg/service/queue"
	"scheduler0/pkg/shared_repo"
	"scheduler0/pkg/utils"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/segmentio/ksuid"
)

type Status struct {
	IsLeader           bool
	IsAuth             bool
	IsAlive            bool
	LastConnectionTime time.Duration
}

type Res struct {
	IsLeader bool
}

type Response struct {
	Data    Res  `json:"data"`
	Success bool `json:"success"`
}

type LogsFetchResponse struct {
	Data    []byte `json:"data"`
	Success bool   `json:"success"`
}

type State int

type BackupRestoreProgress struct {
	OperationType string // "backup" | "restore" | "backup-to-file"
	Status        string // "idle" | "in-progress" | "completed" | "failed"
	Progress      int    // 0-100
	Message       string // Progress message or error
	StartTime     time.Time
	EndTime       time.Time
	BackupPath    string // Result path for backup operations
	sync.RWMutex
}

type nodeService struct {
	// Embedded components
	raftCluster  *raftClusterManager
	peerComm     *peerCommunicator
	eventHandler *eventHandler
	serviceState *serviceState

	// Shared state
	acceptClientWrites    bool
	acceptRequest         bool
	scheduler0RaftStore   fsm.Scheduler0RaftStore
	SingleNodeMode        bool
	ctx                   context.Context
	logger                hclog.Logger
	mtx                   sync.Mutex
	jobProcessor          processor.JobProcessorService
	jobQueue              queue.JobQueueService
	jobExecutor           executor.JobExecutorService
	jobQueuesRepo         job_queue.JobQueuesRepo
	jobRepo               job.JobRepo
	projectRepo           project.ProjectRepo
	jobExecutionRepo      job_execution.JobExecutionsRepo
	sharedRepo            shared_repo.SharedRepo
	asyncTaskRepo         async_task.AsyncTasksRepo
	peerObserverChannels  chan raft.Observation
	asyncTaskManager      async_task_service.AsyncTaskService
	dispatcher            *utils.Dispatcher
	fanIns                sync.Map // models.PeerFanIn
	fanInCh               chan models.PeerFanIn
	completedFanInCh      sync.Map
	scheduler0Config      config.Scheduler0Config
	scheduler0Secrets     secrets.Scheduler0Secrets
	scheduler0RaftActions fsm.Scheduler0RaftActions
	postProcessingChannel chan models.PostProcess
	TransportManager      *raft.NetworkTransport
	LogDb                 *boltdb.BoltStore
	StoreDb               *boltdb.BoltStore
	FileSnapShot          *raft.FileSnapshotStore
	State                 State
	FsmStore              fsm.Scheduler0RaftStore
	isExistingNode        bool
	client                Client
	raftLn                network.Listener
	etcdService           etcd.EtcdService
	peersFromEtcd         []config.RaftNode
	peersMutex            sync.RWMutex
	leadershipDebounce    *utils.Debounce
	latestIsLeader        bool
	latestIsLeaderMtx     sync.Mutex
	sqliteDbExists        bool
	s3Client              *s3.Client
	alertPublisher        alerts.Publisher
}

type NodeService interface {
	RaftClusterManager
	PeerCommunicator
	EventHandler
	ServiceState
	GetJobQueuesRepo() job_queue.JobQueuesRepo
	GetJobExecutor() executor.JobExecutorService
	BackupDatabase(ctx context.Context, requestId string) error
	RestoreDatabase(ctx context.Context, filePath string, requestId string) error
}

func NewNode(
	ctx context.Context,
	logger hclog.Logger,
	scheduler0Config config.Scheduler0Config,
	scheduler0Secrets secrets.Scheduler0Secrets,
	fsmStore fsm.Scheduler0RaftStore,
	fsmActions fsm.Scheduler0RaftActions,
	jobExecutor executor.JobExecutorService,
	jobQueue queue.JobQueueService,
	jobProcessor processor.JobProcessorService,
	jobRepo job.JobRepo,
	sharedRepo shared_repo.SharedRepo,
	jobExecutionRepo job_execution.JobExecutionsRepo,
	asyncTaskRepo async_task.AsyncTasksRepo,
	asyncTaskManager async_task_service.AsyncTaskService,
	dispatcher *utils.Dispatcher,
	postProcessingChannel chan models.PostProcess,
	isExistingNode bool,
	sqliteDbExists bool,
	nodeClient Client,
	raftLn network.Listener,
	etcdService etcd.EtcdService,
	logDb *boltdb.BoltStore,
	storeDb *boltdb.BoltStore,
	fileSnapShot *raft.FileSnapshotStore,
	transportManager *raft.NetworkTransport,
	jobQueuesRepo job_queue.JobQueuesRepo,
	s3Client *s3.Client,
	alertPublisher alerts.Publisher,
) NodeService {
	nodeServiceLogger := logger.Named("node-service")
	configs := scheduler0Config.GetConfigurations()

	// Default peer observer channel size; will be updated dynamically from etcd.
	numReplicas := 10
	if etcdService != nil && len(configs.EtcdEndpoints) > 0 {
		// Try to size the channel based on currently registered peers in etcd.
		if peers, err := etcdService.GetPeers(ctx); err == nil && len(peers) > 0 {
			numReplicas = len(peers)
		}
	}

	nodeServiceLogger.Info("Initializing Node Service")

	node := &nodeService{
		logger:                nodeServiceLogger,
		acceptClientWrites:    false,
		ctx:                   ctx,
		jobProcessor:          jobProcessor,
		jobQueue:              jobQueue,
		jobExecutor:           jobExecutor,
		jobQueuesRepo:         jobQueuesRepo,
		jobRepo:               jobRepo,
		jobExecutionRepo:      jobExecutionRepo,
		asyncTaskRepo:         asyncTaskRepo,
		isExistingNode:        isExistingNode,
		peerObserverChannels:  make(chan raft.Observation, numReplicas),
		asyncTaskManager:      asyncTaskManager,
		dispatcher:            dispatcher,
		fanIns:                sync.Map{},
		fanInCh:               make(chan models.PeerFanIn),
		acceptRequest:         false,
		scheduler0Config:      scheduler0Config,
		scheduler0Secrets:     scheduler0Secrets,
		scheduler0RaftActions: fsmActions,
		scheduler0RaftStore:   fsmStore,
		sharedRepo:            sharedRepo,
		client:                nodeClient,
		raftLn:                raftLn,
		sqliteDbExists:        sqliteDbExists,
		postProcessingChannel: postProcessingChannel,
		etcdService:           etcdService,
		peersFromEtcd:         make([]config.RaftNode, 0),
		LogDb:                 logDb,
		StoreDb:               storeDb,
		FileSnapShot:          fileSnapShot,
		TransportManager:      transportManager,
		leadershipDebounce:    utils.NewDebounce(),
		s3Client:              s3Client,
		alertPublisher:        alertPublisher,
	}

	// Initialize embedded components
	node.serviceState = newServiceState(node)
	node.eventHandler = newEventHandler(node)
	node.peerComm = newPeerCommunicator(node)
	node.raftCluster = newRaftClusterManager(node)

	return node
}

// Delegation methods for NodeService interface - these delegate to embedded components

// RaftClusterManager delegations
func (node *nodeService) Start() {
	node.raftCluster.Start()
}

func (node *nodeService) RemoveSelfFromCluster(ctx context.Context) error {
	return node.raftCluster.RemoveSelfFromCluster(ctx)
}

func (node *nodeService) AddSelfToCluster(ctx context.Context) error {
	return node.raftCluster.AddSelfToCluster(ctx)
}

func (node *nodeService) ForceRebuildCluster(ctx context.Context, seedNodeId uint64) error {
	return node.raftCluster.ForceRebuildCluster(ctx, seedNodeId)
}

func (node *nodeService) ResetRaftState(ctx context.Context) error {
	return node.raftCluster.ResetRaftState(ctx)
}

func (node *nodeService) RemoveNode(ctx context.Context, nodeId uint64) error {
	return node.raftCluster.RemoveNode(ctx, nodeId)
}

func (node *nodeService) AddNode(ctx context.Context, nodeId uint64, nodeAddress string, clientAddress string) error {
	return node.raftCluster.AddNode(ctx, nodeId, nodeAddress, clientAddress)
}

func (node *nodeService) PromoteNode(ctx context.Context, nodeId uint64) error {
	return node.raftCluster.PromoteNode(ctx, nodeId)
}

func (node *nodeService) DemoteNode(ctx context.Context, nodeId uint64) error {
	return node.raftCluster.DemoteNode(ctx, nodeId)
}

func (node *nodeService) TransferLeadership(ctx context.Context) error {
	return node.raftCluster.TransferLeadership(ctx)
}

func (node *nodeService) ListNodes(ctx context.Context) ([]config.RaftNode, error) {
	return node.raftCluster.ListNodes(ctx)
}

func (node *nodeService) GetRaftStats() map[string]string {
	return node.raftCluster.GetRaftStats()
}

func (node *nodeService) GetRaftLeaderWithId() (raft.ServerAddress, raft.ServerID) {
	return node.raftCluster.GetRaftLeaderWithId()
}

// PeerCommunicator delegations
func (node *nodeService) GetUncommittedLogs(requestId string) {
	node.peerComm.GetUncommittedLogs(requestId)
}

func (node *nodeService) ReturnUncommittedLogs(requestId string) {
	node.peerComm.ReturnUncommittedLogs(requestId)
}

// ServiceState delegations
func (node *nodeService) CanAcceptClientWriteRequest() bool {
	return node.serviceState.CanAcceptClientWriteRequest()
}

func (node *nodeService) CanAcceptRequest() bool {
	return node.serviceState.CanAcceptRequest()
}

func (node *nodeService) StopJobs() {
	node.serviceState.StopJobs()
}

func (node *nodeService) StartJobs() {
	node.serviceState.StartJobs()
}

func (node *nodeService) UpdateLocalQuotaAllocations(accountAllocations map[uint64]uint64) error {
	return node.serviceState.UpdateLocalQuotaAllocations(accountAllocations)
}

func (node *nodeService) ResetLocalQuotaAllocations() {
	node.serviceState.ResetLocalQuotaAllocations()
}

func (node *nodeService) GetLocalQuotaAllocations() map[uint64]uint64 {
	return node.serviceState.GetLocalQuotaAllocations()
}

func (node *nodeService) NotifyLeaderAccountExhaustion(accountId uint64) error {
	return node.serviceState.NotifyLeaderAccountExhaustion(accountId)
}

func (node *nodeService) UpdateJobsStatusByAccountId(accountId uint64, status string) error {
	return node.serviceState.UpdateJobsStatusByAccountId(accountId, status)
}

func (node *nodeService) UpdateJobOnLeader(job models.Job) error {
	return node.serviceState.UpdateJobOnLeader(job)
}

// Additional RaftClusterManager internal method delegations
func (node *nodeService) AuthRaftConfiguration() raft.Configuration {
	return node.raftCluster.AuthRaftConfiguration()
}

func (node *nodeService) ReconcileRaftMembershipWithPeers(peers []config.RaftNode) {
	node.raftCluster.ReconcileRaftMembershipWithPeers(peers)
}

func (node *nodeService) HandleRaftLeadershipChanges(isLeader bool) {
	node.raftCluster.HandleRaftLeadershipChanges(isLeader)
}

func (node *nodeService) HandleRaftLeadershipChangesDebounced(isLeader bool) {
	node.raftCluster.HandleRaftLeadershipChangesDebounced(isLeader)
}

func (node *nodeService) HandleRaftObserverChannelChanges(o raft.Observation) {
	node.raftCluster.HandleRaftObserverChannelChanges(o)
}

// Additional PeerCommunicator method delegations
func (node *nodeService) AuthenticateWithPeersFromEtcd() map[string]Status {
	return node.peerComm.AuthenticateWithPeersFromEtcd()
}

func (node *nodeService) WatchPeersFromEtcd() {
	node.peerComm.WatchPeersFromEtcd()
}

func (node *nodeService) GetPeers() []config.RaftNode {
	return node.peerComm.GetPeers()
}

func (node *nodeService) StopAllJobsOnAllWorkerNodes() {
	node.peerComm.StopAllJobsOnAllWorkerNodes()
}

func (node *nodeService) StartJobsOnWorkerNodes() {
	node.peerComm.StartJobsOnWorkerNodes()
}

func (node *nodeService) GetRandomFanInPeerHTTPAddresses(excludeList map[string]bool) []string {
	return node.peerComm.GetRandomFanInPeerHTTPAddresses(excludeList)
}

func (node *nodeService) FanInLocalDataFromPeersSync() {
	node.peerComm.FanInLocalDataFromPeersSync()
}

func (node *nodeService) FanInLocalDataFromPeers() {
	node.peerComm.FanInLocalDataFromPeers()
}

func (node *nodeService) SelectRandomPeersToFanIn() []models.PeerFanIn {
	return node.peerComm.SelectRandomPeersToFanIn()
}

func (node *nodeService) CommitFetchedUnCommittedLogs(peerFanIns []models.PeerFanIn) {
	node.peerComm.CommitFetchedUnCommittedLogs(peerFanIns)
}

// Additional EventHandler method delegations
func (node *nodeService) ListenOnInputQueues() {
	node.eventHandler.ListenOnInputQueues()
}

func (node *nodeService) HandleUncommittedAsyncTasks(asyncTasks []models.AsyncTask) {
	node.eventHandler.HandleUncommittedAsyncTasks(asyncTasks)
}

// Additional ServiceState method delegations
func (node *nodeService) BeginAcceptingClientWriteRequest() {
	node.serviceState.BeginAcceptingClientWriteRequest()
}

func (node *nodeService) StopAcceptingClientWriteRequest() {
	node.serviceState.StopAcceptingClientWriteRequest()
}

// BackupDatabase creates an automatic timestamped backup
func (node *nodeService) BackupDatabase(ctx context.Context, requestId string) error {
	node.dispatcher.NoBlockQueue(func(successChannel chan any, errorChannel chan any) {
		defer func() {
			close(successChannel)
			close(errorChannel)
		}()

		dataStore := node.scheduler0RaftStore.GetDataStore()

		node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskInProgress, "Backup started")

		backupPath, err := dataStore.Backup(ctx)
		if err != nil {
			node.logger.Error("backup failed", "error", err)
			node.alertBackupFailed("sqlite backup failed", err, map[string]any{"stage": "sqlite"})
			node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, err.Error())
			errorChannel <- err
			return
		}
		node.logger.Info("backup path", "path", backupPath)

		// Upload to S3 only if S3Bucket is configured
		configs := node.scheduler0Config.GetConfigurations()
		var msg string
		if configs.S3Bucket != "" {
			s3Key, err := node.UploadBackupToS3(ctx, backupPath)
			if err != nil {
				node.logger.Error("upload backup to S3 failed", "error", err)
				node.alertBackupFailed("backup upload to S3 failed", err, map[string]any{"stage": "s3", "bucket": configs.S3Bucket})
				node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, err.Error())
				errorChannel <- err
				return
			}
			msg = fmt.Sprintf("Backup completed, path: %s, s3Key: %s", backupPath, s3Key)
			node.logger.Info("database backup completed", "path", backupPath, "s3Key", s3Key)
		} else {
			msg = fmt.Sprintf("Backup completed (local only), path: %s", backupPath)
			node.logger.Info("database backup completed (local only)", "path", backupPath)
		}

		node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskSuccess, msg)
		successChannel <- backupPath
	})
	return nil
}

// RestoreDatabase restores the database from a backup file.
// When S3 is configured, fileName is the S3 object key (filename) and the file is downloaded from S3 first.
// When S3 is not configured, fileName is a local file path.
func (node *nodeService) RestoreDatabase(ctx context.Context, fileName string, requestId string) error {
	node.dispatcher.NoBlockQueue(func(successChannel chan any, errorChannel chan any) {
		defer func() {
			close(successChannel)
			close(errorChannel)
		}()

		dataStore := node.scheduler0RaftStore.GetDataStore()
		node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskInProgress, "Restore started")

		configs := node.scheduler0Config.GetConfigurations()
		restorePath := fileName

		if configs.S3Bucket != "" {
			node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskInProgress, "Downloading backup from S3")
			localPath, err := node.downloadBackupFromS3(ctx, fileName)
			if err != nil {
				node.logger.Error("failed to download backup from S3", "error", err, "key", fileName)
				node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, fmt.Sprintf("download from S3: %v", err))
				errorChannel <- err
				return
			}
			defer os.Remove(localPath)
			restorePath = localPath
		}

		if err := dataStore.Restore(ctx, restorePath); err != nil {
			node.logger.Error("restore failed", "error", err)
			node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, err.Error())
			errorChannel <- err
			return
		}

		node.logger.Info("sqlite db exists, creating snapshot from sqlite db")
		snapshot := fsm.NewFSMSnapshot(dataStore)
		rCfg := node.scheduler0RaftStore.GetRaft().GetConfiguration().Configuration()
		sink, err := node.FileSnapShot.Create(1, 1, 1, rCfg, 1, node.TransportManager)
		if err != nil {
			node.logger.Error("failed to create snapshot sink from sqlite db on bootstrap", "error", err)
			node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, fmt.Sprintf("snapshot sink: %v", err))
			errorChannel <- err
			return
		}
		node.logger.Info("snapshot sink created from sqlite db on bootstrap")

		if err := snapshot.Persist(sink); err != nil {
			node.logger.Error("failed to persist snapshot from sqlite db on bootstrap", "error", err)
			_ = sink.Close()
			node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, fmt.Sprintf("snapshot persist: %v", err))
			errorChannel <- err
			return
		}
		node.logger.Info("snapshot persisted from sqlite db on bootstrap")

		if err := sink.Close(); err != nil {
			node.logger.Error("failed to close snapshot sink from sqlite db on bootstrap", "error", err)
			node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskFail, fmt.Sprintf("snapshot close: %v", err))
			errorChannel <- err
			return
		}
		node.logger.Info("snapshot sink closed from sqlite db on bootstrap")

		node.asyncTaskManager.UpdateTasksByRequestId(requestId, models.AsyncTaskSuccess, "Restore completed")
		node.logger.Info("database restore completed", "from", fileName)
		successChannel <- true
	})

	return nil
}

// downloadBackupFromS3 downloads the object with the given key from S3 to a temp file and returns its path.
func (node *nodeService) downloadBackupFromS3(ctx context.Context, s3Key string) (string, error) {
	configs := node.scheduler0Config.GetConfigurations()
	if configs.S3Bucket == "" {
		return "", fmt.Errorf("S3Bucket is not defined")
	}

	out, err := node.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(configs.S3Bucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return "", fmt.Errorf("get object: %w", err)
	}
	defer out.Body.Close()

	tmpFile, err := os.CreateTemp("", "scheduler0-restore-*.db")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	_, err = io.Copy(tmpFile, out.Body)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close temp file: %w", err)
	}

	node.logger.Info("downloaded backup from S3", "bucket", configs.S3Bucket, "key", s3Key, "path", tmpPath)
	return tmpPath, nil
}

func (node *nodeService) BeginAcceptingClientRequest() {
	node.serviceState.BeginAcceptingClientRequest()
}

func (node *nodeService) GetJobQueuesRepo() job_queue.JobQueuesRepo {
	return node.jobQueuesRepo
}

func (node *nodeService) GetJobExecutor() executor.JobExecutorService {
	return node.jobExecutor
}

// alertBackupFailed publishes backup_failed to the ops alert topic. Fire-and-
// forget: backup failure handling never waits on SNS.
func (node *nodeService) alertBackupFailed(summary string, cause error, details map[string]any) {
	if node.alertPublisher == nil {
		return
	}
	configs := node.scheduler0Config.GetConfigurations()
	if details == nil {
		details = map[string]any{}
	}
	details["error"] = cause
	details["nodeId"] = configs.NodeId
	go func() {
		if err := node.alertPublisher.Publish(context.Background(), alerts.Alert{
			Event:    alerts.EventBackupFailed,
			Severity: alerts.SeverityError,
			Summary:  summary + ": " + cause.Error(),
			Details:  details,
		}); err != nil {
			node.logger.Error("ops alert publish failed", "event", alerts.EventBackupFailed, "error", err)
		}
	}()
}

// UploadBackupToS3 uploads backupPath to S3 and returns the object key used.
func (node *nodeService) UploadBackupToS3(ctx context.Context, backupPath string) (string, error) {
	configs := node.scheduler0Config.GetConfigurations()
	if configs.S3Bucket == "" {
		return "", fmt.Errorf("S3Bucket is not defined")
	}

	file, err := os.Open(backupPath)
	if err != nil {
		return "", fmt.Errorf("open backup file: %w", err)
	}
	defer file.Close()

	key := fmt.Sprintf("%d-%s-%s", configs.NodeId, ksuid.New().String(), filepath.Base(backupPath))

	node.logger.Info("uploading backup to S3", "bucket", configs.S3Bucket, "key", key, "region", configs.AWSRegion)

	_, err = node.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(configs.S3Bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return "", fmt.Errorf("upload backup to S3: %w", err)
	}
	node.logger.Info("backup uploaded to S3", "bucket", configs.S3Bucket, "key", key)
	return key, nil
}
