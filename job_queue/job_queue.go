package job_queue

import (
	"encoding/json"
	"github.com/hashicorp/raft"
	"log"
	"math"
	"scheduler0/constants"
	"scheduler0/job_executor"
	"scheduler0/marsher"
	"scheduler0/models"
	"scheduler0/protobuffs"
	"scheduler0/utils"
	"strconv"
	"time"
)

type JobQueueCommand struct {
	job      models.JobModel
	serverId string
}

type jobQueue struct {
	Executor *job_executor.JobExecutor
	raft     *raft.Raft
	logger   *log.Logger
}

type JobQueue interface {
	Queue(jobs []models.JobModel)
}

func NewJobQueue(logger *log.Logger, raft *raft.Raft, Executor *job_executor.JobExecutor) JobQueue {
	return &jobQueue{
		Executor: Executor,
		raft:     raft,
		logger:   logger,
	}
}

func (jobQ *jobQueue) Queue(jobs []models.JobModel) {
	f := jobQ.raft.VerifyLeader()
	if f.Error() != nil {
		jobQ.logger.Println("skipping job queueing as node is not the leader")
		return
	}

	conf := jobQ.raft.GetConfiguration().Configuration()
	servers := conf.Servers

	if len(servers) == 1 {
		jobQ.Executor.Run(jobs)
		return
	}

	configs := utils.GetScheduler0Configurations(jobQ.logger)
	hasSplit := ((len(jobs) - 1) % 2) == 1
	batchPerPeer := math.Floor(float64((len(jobs) - 1) / (len(servers) - 1)))
	batchRanges := [][]int64{}
	var currentRange int64 = 0

	for len(batchRanges) < len(servers)-1 {
		batchRanges = append(batchRanges, []int64{
			currentRange,
			currentRange + int64(batchPerPeer),
		})
		currentRange += int64(batchPerPeer) + 1
	}

	if hasSplit {
		batchRanges[len(batchRanges)-1] = []int64{
			batchRanges[len(batchRanges)-1][0],
			int64(len(jobs)),
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
			d, err := json.Marshal(jobs[batchRange[0]:batchRange[1]])
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

			jobQ.logger.Println("Queueing jobs ", len(jobs[batchRange[0]:batchRange[1]]), " on server ", createCommand.Sql, " s: ", s, " j: ", j)
			af := jobQ.raft.Apply(createCommandData, time.Second*time.Duration(timeout)).(raft.ApplyFuture)
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
