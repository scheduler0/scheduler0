package job_queue

import (
	"encoding/json"
	"github.com/hashicorp/raft"
	"log"
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
}

type JobQueue interface {
	Queue(jobs []models.JobModel)
}

func NewJobQueue(raft *raft.Raft, Executor *job_executor.JobExecutor) *jobQueue {
	return &jobQueue{
		Executor: Executor,
		raft:     raft,
	}
}

func (jobQ *jobQueue) Queue(jobs []models.JobModel) {
	f := jobQ.raft.VerifyLeader()
	if f.Error() != nil {
		utils.Info("skipping job queueing as node is not the leader")
	}

	conf := jobQ.raft.GetConfiguration().Configuration()
	servers := conf.Servers

	if len(servers) == 1 {
		jobQ.Executor.Run(jobs)
		return
	}

	configs := utils.GetScheduler0Configurations()

	timeout, err := strconv.Atoi(configs.RaftApplyTimeout)
	if err != nil {
		log.Fatalln(err.Error())
	}

	s := 0
	j := 0

	for j < len(jobs) {
		server := servers[s]
		job := jobs[j]

		if string(server.Address) == configs.RaftAddress {
			s++
		} else {
			d, err := json.Marshal(job)
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

			utils.Info("Queueing job ", job.ID, " on server ", createCommand.Sql, " s: ", s, " j: ", j)
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
