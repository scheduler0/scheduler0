package service

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"log"
	"math"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/utils"
	"sync"
)

type JobQueueCommand struct {
	job      models.Job
	serverId string
}

type JobQueue struct {
	SingleNodeMode        bool
	jobsQueueRepo         repository.JobQueuesRepo
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

func NewJobQueue(ctx context.Context, logger hclog.Logger, scheduler0Config config.Scheduler0Config, scheduler0RaftActions fsm.Scheduler0RaftActions, fsm fsm.Scheduler0RaftStore, jobsQueueRepo repository.JobQueuesRepo) *JobQueue {
	return &JobQueue{
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

func (jobQ *JobQueue) AddServers(nodeIds []uint64) {
	jobQ.mtx.Lock()
	defer jobQ.mtx.Unlock()

	for _, nodeId := range nodeIds {
		jobQ.allocations[nodeId] = 0
	}
}

func (jobQ *JobQueue) RemoveServers(nodeIds []uint64) {
	jobQ.mtx.Lock()
	defer jobQ.mtx.Unlock()

	for _, nodeId := range nodeIds {
		delete(jobQ.allocations, nodeId)
	}
}

func (jobQ *JobQueue) Queue(jobs []models.Job) {
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

func (jobQ *JobQueue) IncrementQueueVersion() {
	jobQ.mtx.Lock()
	defer jobQ.mtx.Unlock()

	lastVersion := jobQ.jobsQueueRepo.GetLastVersion()
	_, err := jobQ.scheduler0RaftActions.WriteCommandToRaftLog(
		jobQ.fsm.GetRaft(),
		constants.CommandTypeDbExecute,
		fmt.Sprintf("insert into %s (%s, %s) values (?, ?)",
			repository.JobQueuesVersionTableName,
			repository.JobQueueVersion,
			repository.JobNumberOfActiveNodesVersion,
		), 0, []interface{}{lastVersion + 1, len(jobQ.allocations)})
	if err != nil {
		log.Fatalln("failed to increment job queue version", err)
	}
}

func (jobQ *JobQueue) queue(minId, maxId int64) {
	f := jobQ.fsm.GetRaft().VerifyLeader()
	if f.Error() != nil {
		jobQ.logger.Error("skipping job queueing as node is not the leader")
		return
	}

	serverAllocations := jobQ.assignJobRangeToServers(minId, maxId)
	lastVersion := jobQ.jobsQueueRepo.GetLastVersion()
	j := 0

	for j < len(serverAllocations) {
		server := jobQ.getNextServerToQueue()

		batchRange := []interface{}{
			serverAllocations[j][0],
			serverAllocations[j][1],
			lastVersion,
		}

		_, err := jobQ.scheduler0RaftActions.WriteCommandToRaftLog(
			jobQ.fsm.GetRaft(),
			constants.CommandTypeJobQueue,
			"",
			server,
			batchRange,
		)

		if err != nil {
			if err == raft.ErrNotLeader {
				log.Fatalln("raft leader not found")
			}
			log.Fatalln("failed to queue jobs: raft apply error: ", err)
		}
		jobQ.allocations[server] += serverAllocations[j][1] - serverAllocations[j][0]
		j++
	}
}

func (jobQ *JobQueue) getNextServerToQueue() uint64 {
	configs := jobQ.schedulerOConfig.GetConfigurations()

	var minAllocation uint64 = math.MaxInt64
	var minServer uint64

	// Edge case for single node mode
	if jobQ.SingleNodeMode {
		return configs.NodeId
	}

	for server, allocation := range jobQ.allocations {
		if allocation < minAllocation && server != configs.NodeId {
			minAllocation = allocation
			minServer = server
		}
	}

	return minServer
}

func (jobQ *JobQueue) assignJobRangeToServers(minId, maxId int64) [][]uint64 {
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

			if epoc == maxId {
				break
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

func (jobQ *JobQueue) GetJobAllocations() map[uint64]uint64 {
	jobQ.mtx.Lock()
	defer jobQ.mtx.Unlock()

	return jobQ.allocations
}
