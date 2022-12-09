package job_queue

import (
	"encoding/json"
	"github.com/hashicorp/raft"
	"google.golang.org/protobuf/proto"
	"log"
	"math"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/job_executor"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"scheduler0/utils"
	"strconv"
	"sync"
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
	minId       int64
	maxId       int64
	threshold   time.Time
	ticker      *time.Ticker
	mtx         sync.Mutex
	once        sync.Once
}

type JobQueue interface {
	Queue(jobs []models.JobModel)
	AddServers(serverAddresses []raft.Server)
	RemoveServers(serverAddresses []raft.Server)
}

func NewJobQueue(logger *log.Logger, fsm *fsm.Store, Executor *job_executor.JobExecutor) *jobQueue {
	return &jobQueue{
		Executor:    Executor,
		fsm:         fsm,
		logger:      logger,
		minId:       math.MaxInt64,
		maxId:       math.MinInt16,
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
	jobQ.mtx.Lock()
	defer jobQ.mtx.Unlock()

	if len(jobs) < 1 {
		return
	}

	if len(jobs) > 1 {
		for _, job := range jobs {
			if jobQ.maxId < job.ID {
				jobQ.maxId = job.ID
			}
			if jobQ.minId > job.ID {
				jobQ.minId = job.ID
			}
		}
	} else {
		jobQ.maxId = jobs[0].ID
		jobQ.minId = jobs[0].ID
	}

	configs := config.GetConfigurations(jobQ.logger)
	jobQ.threshold = time.Now().Add(time.Duration(configs.JobPrepareDebounceDelay) * time.Second)

	jobQ.once.Do(func() {
		go func() {
			defer func() {
				jobQ.mtx.Lock()
				jobQ.ticker.Stop()
				jobQ.once = sync.Once{}
				jobQ.minId = math.MaxInt64
				jobQ.maxId = math.MinInt16
				jobQ.mtx.Unlock()
			}()

			jobQ.ticker = time.NewTicker(time.Duration(500) * time.Millisecond)

			for {
				select {
				case <-jobQ.ticker.C:
					jobQ.mtx.Lock()
					if time.Now().After(jobQ.threshold) {
						jobQ.queue(jobQ.minId, jobQ.maxId)
						jobQ.minId = math.MaxInt64
						jobQ.maxId = math.MinInt16
						jobQ.mtx.Unlock()
						return
					}
					jobQ.mtx.Unlock()
				}
			}
		}()
	})
}

func (jobQ *jobQueue) queue(minId, maxId int64) {
	f := jobQ.fsm.Raft.VerifyLeader()
	if f.Error() != nil {
		jobQ.logger.Println("skipping job queueing as node is not the leader")
		return
	}

	configs := config.GetConfigurations(jobQ.logger)

	if len(jobQ.allocations) == 1 {
		jobQ.Executor.QueueExecutions([]interface{}{configs.RaftAddress, minId, maxId})
		return
	}

	batchRanges := [][]int64{}

	numberOfServers := len(jobQ.allocations) - 1
	for i := 0; i < numberOfServers; i++ {
		batchRanges = append(batchRanges, []int64{})
	}

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

	timeout, err := strconv.Atoi(configs.RaftApplyTimeout)
	if err != nil {
		jobQ.logger.Fatalln(err.Error())
	}

	allocations := jobQ.allocations

	j := 0
	for j < len(batchRanges) {
		if utils.MonitorMemoryUsage(jobQueue{}.logger) {
			return
		}

		server := func() raft.ServerAddress {
			var minAllocation int64 = math.MaxInt64
			var minServer raft.ServerAddress
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
		}

		d, err := json.Marshal(batchRange)
		if err != nil {
			log.Fatalln(err.Error())
		}

		createCommand := &protobuffs.Command{
			Type: protobuffs.Command_Type(constants.CommandTypeJobQueue),
			Sql:  string(server),
			Data: d,
		}

		createCommandData, err := proto.Marshal(createCommand)
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
		allocations[server] += int64(len(batchRange))
		j++
	}

	jobQ.allocations = allocations
}
