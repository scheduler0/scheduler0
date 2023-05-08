package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"google.golang.org/protobuf/proto"
	"log"
	"math"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"scheduler0/repository"
	"scheduler0/service/executor"
	"scheduler0/utils"
	"sync"
	"time"
)

type JobQueueCommand struct {
	job      models.JobModel
	serverId string
}

type JobQueue struct {
	Executor              *executor.JobExecutor
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

func NewJobQueue(ctx context.Context, logger hclog.Logger, scheduler0Config config.Scheduler0Config, scheduler0RaftActions fsm.Scheduler0RaftActions, fsm fsm.Scheduler0RaftStore, Executor *executor.JobExecutor, jobsQueueRepo repository.JobQueuesRepo) *JobQueue {
	return &JobQueue{
		Executor:              Executor,
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
	for _, nodeId := range nodeIds {
		jobQ.allocations[nodeId] = nodeId
	}
}

func (jobQ *JobQueue) RemoveServers(nodeIds []uint64) {
	for _, nodeId := range nodeIds {
		delete(jobQ.allocations, nodeId)
	}
}

func (jobQ *JobQueue) Queue(jobs []models.JobModel) {
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

func (jobQ *JobQueue) queue(minId, maxId int64) {
	f := jobQ.fsm.GetRaft().VerifyLeader()
	if f.Error() != nil {
		jobQ.logger.Error("skipping job queueing as node is not the leader")
		return
	}

	numberOfServers := len(jobQ.allocations) - 1
	var serverAllocations [][]uint64

	if numberOfServers > 0 {
		currentServer := 0
		cycle := int64(math.Ceil(float64((maxId - minId) / int64(numberOfServers))))
		epoc := minId
		for epoc <= maxId {
			if len(serverAllocations) <= currentServer {
				serverAllocations = append(serverAllocations, []uint64{})
			}

			lowerBound := epoc
			upperBound := epoc + cycle
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
	} else {
		serverAllocations = append(serverAllocations, []uint64{uint64(minId), uint64(maxId)})
	}

	lastVersion := jobQ.jobsQueueRepo.GetLastVersion()
	configs := jobQ.schedulerOConfig.GetConfigurations()

	j := 0
	for j < len(serverAllocations) {
		server := jobQ.getServerToQueue()

		batchRange := []interface{}{
			serverAllocations[j][0],
			serverAllocations[j][len(serverAllocations[j])-1],
			lastVersion,
		}

		d, err := json.Marshal(batchRange)
		if err != nil {
			log.Fatalln(err.Error())
		}

		createCommand := &protobuffs.Command{
			Type:       protobuffs.Command_Type(constants.CommandTypeJobQueue),
			Sql:        "",
			Data:       d,
			TargetNode: server,
		}

		createCommandData, err := proto.Marshal(createCommand)
		if err != nil {
			log.Fatalln(err.Error())
		}

		af := jobQ.fsm.GetRaft().Apply(createCommandData, time.Second*time.Duration(configs.RaftApplyTimeout)).(raft.ApplyFuture)
		if af.Error() != nil {
			if af.Error() == raft.ErrNotLeader {
				log.Fatalln("raft leader not found")
			}
			log.Fatalln(af.Error().Error())
		}
		jobQ.allocations[server] += uint64(len(batchRange))
		j++
	}
}

func (jobQ *JobQueue) getServerToQueue() uint64 {
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

func (jobQ *JobQueue) IncrementQueueVersion() {
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
