package queue

import (
	"context"
	"github.com/hashicorp/go-hclog"
	"math"
	"scheduler0/pkg/config"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/repository/job_queue"
	"scheduler0/pkg/utils"
	"sync"
)

type JobQueueCommand struct {
	job      models.Job
	serverId string
}

type jobQueue struct {
	singleNodeMode        bool
	jobsQueueRepo         job_queue.JobQueuesRepo
	fsm                   fsm.Scheduler0RaftStore
	logger                hclog.Logger
	allocations           map[uint64]uint64
	minId                 int64
	maxId                 int64
	mtx                   sync.Mutex
	once                  sync.Once
	debounce              *utils.Debounce
	context               context.Context
	schedulerOConfig      config.Scheduler0Config
	scheduler0RaftActions fsm.Scheduler0RaftActions
}

//go:generate mockery --name JobQueueService --output ./ --inpackage
type JobQueueService interface {
	AddServers(nodeIds []uint64)
	RemoveServers(nodeIds []uint64)
	Queue(jobs []models.Job)
	IncrementQueueVersion()
	GetJobAllocations() map[uint64]uint64
	SetSingleNodeMode(singleNodeMode bool)
	GetSingleNodeMode() bool
}

func NewJobQueue(ctx context.Context, logger hclog.Logger, scheduler0Config config.Scheduler0Config, scheduler0RaftActions fsm.Scheduler0RaftActions, fsm fsm.Scheduler0RaftStore, jobsQueueRepo job_queue.JobQueuesRepo) JobQueueService {
	return &jobQueue{
		jobsQueueRepo:         jobsQueueRepo,
		context:               ctx,
		fsm:                   fsm,
		logger:                logger.Named("job-queue-service"),
		minId:                 math.MaxInt64,
		maxId:                 math.MinInt64,
		allocations:           map[uint64]uint64{},
		debounce:              utils.NewDebounce(),
		schedulerOConfig:      scheduler0Config,
		scheduler0RaftActions: scheduler0RaftActions,
	}
}

func (jobQ *jobQueue) AddServers(nodeIds []uint64) {
	jobQ.mtx.Lock()
	defer jobQ.mtx.Unlock()

	for _, nodeId := range nodeIds {
		jobQ.allocations[nodeId] = 0
	}
}

func (jobQ *jobQueue) RemoveServers(nodeIds []uint64) {
	jobQ.mtx.Lock()
	defer jobQ.mtx.Unlock()

	for _, nodeId := range nodeIds {
		delete(jobQ.allocations, nodeId)
	}
}

func (jobQ *jobQueue) Queue(jobs []models.Job) {
	jobQ.mtx.Lock()
	defer jobQ.mtx.Unlock()

	if len(jobs) < 1 {
		return
	}

	for _, job := range jobs {
		if jobQ.maxId < int64(job.ID) {
			jobQ.maxId = int64(job.ID)
		}
		if jobQ.minId > int64(job.ID) {
			jobQ.minId = int64(job.ID)
		}
	}

	jobQ.queue(jobQ.minId, jobQ.maxId)
	jobQ.minId = math.MaxInt64
	jobQ.maxId = math.MinInt16
}

func (jobQ *jobQueue) GetJobAllocations() map[uint64]uint64 {
	jobQ.mtx.Lock()
	defer jobQ.mtx.Unlock()

	return jobQ.allocations
}

func (jobQ *jobQueue) SetSingleNodeMode(singleNodeMode bool) {
	jobQ.singleNodeMode = singleNodeMode
}

func (jobQ *jobQueue) GetSingleNodeMode() bool {
	return jobQ.singleNodeMode
}

func (jobQ *jobQueue) IncrementQueueVersion() {
	jobQ.jobsQueueRepo.IncrementQueueVersion(len(jobQ.allocations))
}

func (jobQ *jobQueue) queue(minId, maxId int64) {
	f := jobQ.fsm.VerifyLeader()
	if f.Error() != nil {
		jobQ.logger.Error("skipping job queueing as node is not the leader")
		return
	}

	serverAllocations := jobQ.assignJobRangeToServers(minId, maxId)
	lastVersion := jobQ.jobsQueueRepo.GetLastVersion()
	j := 0
	jobQueueLogs := []models.JobQueueLog{}

	if jobQ.singleNodeMode {
		jobQueueLogs = append(jobQueueLogs, models.JobQueueLog{
			NodeId:          config.NewScheduler0Config().GetConfigurations().NodeId,
			LowerBoundJobId: uint64(minId),
			UpperBoundJobId: uint64(maxId),
			Version:         lastVersion,
		})
	} else {
		for j < len(serverAllocations) {
			server := jobQ.getNextServerToQueue()
			jobQueueLogs = append(jobQueueLogs, models.JobQueueLog{
				NodeId:          server,
				LowerBoundJobId: serverAllocations[j][0],
				UpperBoundJobId: serverAllocations[j][1],
				Version:         lastVersion,
			})
			jobQ.allocations[server] += serverAllocations[j][1] - serverAllocations[j][0]
			j++
		}
	}

	jobQ.jobsQueueRepo.InsertJobQueueLogs(jobQueueLogs)
}

func (jobQ *jobQueue) getNextServerToQueue() uint64 {
	configs := jobQ.schedulerOConfig.GetConfigurations()

	var minAllocation uint64 = math.MaxInt64
	var minServer uint64

	// Edge case for single node mod

	for server, allocation := range jobQ.allocations {
		if allocation < minAllocation && server != configs.NodeId {
			minAllocation = allocation
			minServer = server
		}
	}

	return minServer
}

func (jobQ *jobQueue) assignJobRangeToServers(minId, maxId int64) [][]uint64 {
	numberOfServers := len(jobQ.allocations) - 1
	var serverAllocations [][]uint64

	// Single server case
	if numberOfServers == 0 {
		serverAllocations = append(serverAllocations, []uint64{uint64(minId), uint64(maxId)})
	}

	if numberOfServers > 0 {
		currentServer := 0

		cycle := int64(math.Ceil(float64((maxId - minId) / int64(numberOfServers))))
		epoc := minId

		for epoc <= maxId {
			if len(serverAllocations) <= currentServer {
				serverAllocations = append(serverAllocations, []uint64{})
			}

			lowerBound := epoc
			upperBound := int64(math.Min(float64(epoc+cycle), float64(maxId)))

			serverAllocations[currentServer] = append(serverAllocations[currentServer], uint64(lowerBound))
			serverAllocations[currentServer] = append(serverAllocations[currentServer], uint64(upperBound))

			epoc = upperBound + 1

			if maxId-upperBound < cycle {
				upperBound = maxId
			}

			currentServer += 1
			if currentServer == numberOfServers {
				currentServer = 0
			}
		}
	}

	return serverAllocations
}
