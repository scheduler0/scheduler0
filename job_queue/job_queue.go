package job_queue

import (
	"encoding/json"
	"github.com/hashicorp/raft"
	"log"
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
	Executor *job_executor.JobExecutor
	fsm      *fsm.Store
	logger   *log.Logger
}

type JobQueue interface {
	Queue(jobs []models.JobModel)
}

func NewJobQueue(logger *log.Logger, fsm *fsm.Store, Executor *job_executor.JobExecutor) JobQueue {
	return &jobQueue{
		Executor: Executor,
		fsm:      fsm,
		logger:   logger,
	}
}

func (jobQ *jobQueue) Queue(jobs []models.JobModel) {
	f := jobQ.fsm.Raft.VerifyLeader()
	if f.Error() != nil {
		jobQ.logger.Println("skipping job queueing as node is not the leader")
		return
	}

	conf := jobQ.fsm.Raft.GetConfiguration().Configuration()
	servers := conf.Servers

	if len(servers) == 1 {
		jobQ.Executor.Run(jobs)
		return
	}

	configs := config.GetScheduler0Configurations(jobQ.logger)
	batchRanges := [][]models.JobModel{}

	numberOfServers := len(servers) - 1

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
		log.Fatalln(err.Error())
	}

	s := 0
	j := 0

	for j < len(batchRanges) {
		server := servers[s]
		batchRange := batchRanges[j]

		if string(server.Address) == configs.RaftAddress {
			s++
		} else {
			d, err := json.Marshal(batchRange)
			if err != nil {
				log.Fatalln(err.Error())
			}

			createCommand := &protobuffs.Command{
				Type: protobuffs.Command_Type(constants.CommandTypeJobQueue),
				Sql:  string(server.Address),
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

			s++
			if s == len(servers) {
				s = 0
			}
			j++
		}
	}
}
