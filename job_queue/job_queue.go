package job_queue

import (
	"encoding/json"
	"github.com/hashicorp/raft"
	"log"
	"math"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/job_executor"
	"scheduler0/marsher"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"strconv"
	"time"
)

type JobQueueCommand struct {
	job      models.JobModel
	serverId string
}

type jobQueue struct {
	Executor    *job_executor.JobExecutor
	fsm         *fsm.Store
	logger      *log.Logger
	allocations map[raft.ServerAddress]int64
}

type JobQueue interface {
	Queue(jobs []models.JobModel)
	AddServers(serverAddresses []raft.Server)
	RemoveServers(serverAddresses []raft.Server)
}

func NewJobQueue(logger *log.Logger, fsm *fsm.Store, Executor *job_executor.JobExecutor) JobQueue {
	return &jobQueue{
		Executor:    Executor,
		fsm:         fsm,
		logger:      logger,
		allocations: map[raft.ServerAddress]int64{},
	}
}

func (jobQ *jobQueue) AddServers(serverAddresses []raft.Server) {
	for _, serverAddress := range serverAddresses {
		jobQ.allocations[serverAddress.Address] = 0
	}
}

func (jobQ *jobQueue) RemoveServers(serverAddresses []raft.Server) {
	for _, serverAddress := range serverAddresses {
		delete(jobQ.allocations, serverAddress.Address)
	}
}

func (jobQ *jobQueue) Queue(jobs []models.JobModel) {
	f := jobQ.fsm.Raft.VerifyLeader()
	if f.Error() != nil {
		jobQ.logger.Println("skipping job queueing as node is not the leader")
		return
	}

	if len(jobQ.allocations) == 1 {
		jobQ.Executor.Run(jobs)
		return
	}

	configs := config.GetScheduler0Configurations(jobQ.logger)
	batchRanges := [][]models.JobModel{}

	numberOfServers := len(jobQ.allocations) - 1
	for i := 0; i < numberOfServers; i++ {
		batchRanges = append(batchRanges, []models.JobModel{})
	}

	currentServer := 0
	for _, job := range jobs {
		batchRanges[currentServer] = append(batchRanges[currentServer], job)
		currentServer += 1
		if currentServer == numberOfServers {
			currentServer = 0
		}
	}

	timeout, err := strconv.Atoi(configs.RaftApplyTimeout)
	if err != nil {
		jobQ.logger.Fatalln(err.Error())
	}

	j := 0
	for j < len(batchRanges) {
		server := func() raft.ServerAddress {
			var minAllocation int64 = math.MinInt16
			var minServer raft.ServerAddress
			for server, allocation := range jobQ.allocations {
				if allocation > minAllocation && string(server) != configs.RaftAddress {
					minAllocation = allocation
					minServer = server
				}
			}
			return minServer
		}()

		batchRange := batchRanges[j]

		d, err := json.Marshal(batchRange)
		if err != nil {
			log.Fatalln(err.Error())
		}

		createCommand := &protobuffs.Command{
			Type: protobuffs.Command_Type(constants.CommandTypeJobQueue),
			Sql:  string(server),
			Data: d,
		}

		createCommandData, err := marsher.MarshalCommand(createCommand)
		if err != nil {
			log.Fatalln(err.Error())
		}

		af := jobQ.fsm.Raft.Apply(createCommandData, time.Second*time.Duration(timeout)).(raft.ApplyFuture)
		if af.Error() != nil {
			if af.Error() == raft.ErrNotLeader {
				log.Fatalln("raft leader not found")
			}
			log.Fatalln(af.Error().Error())
		}
	}
}
