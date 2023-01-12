package queue

import (
	"context"
	"encoding/json"
	"fmt"
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
	Executor       *executor.JobExecutor
	SingleNodeMode bool
	jobsQueueRepo  repository.JobQueuesRepo
	fsm            *fsm.Store
	logger         *log.Logger
	allocations    map[raft.ServerAddress]int64
	minId          int64
	maxId          int64
	mtx            sync.Mutex
	once           sync.Once
	debounce       *utils.Debounce
	context        context.Context
}

func NewJobQueue(ctx context.Context, logger *log.Logger, fsm *fsm.Store, Executor *executor.JobExecutor, jobsQueueRepo repository.JobQueuesRepo) *JobQueue {
	return &JobQueue{
		Executor:      Executor,
		jobsQueueRepo: jobsQueueRepo,
		context:       ctx,
		fsm:           fsm,
		logger:        logger,
		minId:         math.MaxInt64,
		maxId:         math.MinInt16,
		allocations:   map[raft.ServerAddress]int64{},
		debounce:      utils.NewDebounce(),
	}
}

func (jobQ *JobQueue) AddServers(serverAddresses []raft.Server) {
	for _, serverAddress := range serverAddresses {
		jobQ.allocations[serverAddress.Address] = 0
	}
}

func (jobQ *JobQueue) RemoveServers(serverAddresses []raft.Server) {
	for _, serverAddress := range serverAddresses {
		delete(jobQ.allocations, serverAddress.Address)
	}
}

func (jobQ *JobQueue) Queue(jobs []models.JobModel) {
	jobQ.mtx.Lock()
	defer jobQ.mtx.Unlock()

	if len(jobs) < 1 {
		return
	}

	for _, job := range jobs {
		if jobQ.maxId < job.ID {
			jobQ.maxId = job.ID
		}
		if jobQ.minId > job.ID {
			jobQ.minId = job.ID
		}
	}

	configs := config.GetConfigurations(jobQ.logger)
	jobQ.debounce.Debounce(jobQ.context, configs.JobQueueDebounceDelay, func() {
		jobQ.mtx.Lock()
		defer jobQ.mtx.Unlock()

		jobQ.queue(jobQ.minId, jobQ.maxId)
		jobQ.minId = math.MaxInt64
		jobQ.maxId = math.MinInt16
	})
}

func (jobQ *JobQueue) queue(minId, maxId int64) {
	f := jobQ.fsm.Raft.VerifyLeader()
	if f.Error() != nil {
		jobQ.logger.Println("skipping job queueing as node is not the leader")
		return
	}

	configs := config.GetConfigurations(jobQ.logger)

	batchRanges := [][]int64{}

	numberOfServers := len(jobQ.allocations) - 1
	for i := 0; i < numberOfServers; i++ {
		batchRanges = append(batchRanges, []int64{})
	}

	if numberOfServers > 0 {
		currentServer := 0
		cycle := int64(math.Ceil(float64((maxId - minId) / int64(numberOfServers))))
		epoc := minId
		for epoc < maxId {
			lowerBound := epoc
			upperBound := epoc + cycle

			batchRanges[currentServer] = append(batchRanges[currentServer], lowerBound)
			batchRanges[currentServer] = append(batchRanges[currentServer], upperBound)

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
		batchRanges = append(batchRanges, []int64{minId, maxId})
	}

	allocations := jobQ.allocations

	lastVersion := jobQ.jobsQueueRepo.GetLastVersion()

	j := 0
	for j < len(batchRanges) {
		server := func() raft.ServerAddress {
			var minAllocation int64 = math.MaxInt64
			var minServer raft.ServerAddress

			// Edge case for single node mode
			if jobQ.SingleNodeMode {
				return raft.ServerAddress(configs.RaftAddress)
			}

			for server, allocation := range allocations {
				if allocation < minAllocation && string(server) != configs.RaftAddress {
					minAllocation = allocation
					minServer = server
				}
			}
			return minServer
		}()

		batchRange := []interface{}{
			batchRanges[j][0],
			batchRanges[j][len(batchRanges[j])-1],
			lastVersion,
		}

		d, err := json.Marshal(batchRange)
		if err != nil {
			log.Fatalln(err.Error())
		}

		createCommand := &protobuffs.Command{
			Type:         protobuffs.Command_Type(constants.CommandTypeJobQueue),
			Sql:          string(server),
			Data:         d,
			ActionTarget: string(server),
		}

		createCommandData, err := proto.Marshal(createCommand)
		if err != nil {
			log.Fatalln(err.Error())
		}

		af := jobQ.fsm.Raft.Apply(createCommandData, time.Second*time.Duration(configs.RaftApplyTimeout)).(raft.ApplyFuture)
		if af.Error() != nil {
			if af.Error() == raft.ErrNotLeader {
				log.Fatalln("raft leader not found")
			}
			log.Fatalln(af.Error().Error())
		}
		allocations[server] += int64(len(batchRange))
		j++
	}

	jobQ.allocations = allocations
}

func (jobQ *JobQueue) IncrementQueueVersion() {
	lastVersion := jobQ.jobsQueueRepo.GetLastVersion()
	_, err := fsm.AppApply(
		jobQ.logger,
		jobQ.fsm.Raft,
		constants.CommandTypeDbExecute,
		fmt.Sprintf("insert into %s (%s, %s) values (?, ?)",
			repository.JobQueuesVersionTableName,
			repository.JobQueueVersion,
			repository.JobNumberOfActiveNodesVersion,
		), []interface{}{lastVersion + 1, len(jobQ.allocations)})
	if err != nil {
		log.Fatalln("failed to increment job queue version", err)
	}
}
