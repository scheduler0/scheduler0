package job_queue

import (
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"log"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/scheduler0time"
	"scheduler0/utils"
	"time"
)

const (
	JobQueuesTableName        = "job_queues"
	JobQueuesVersionTableName = "job_queue_versions"
)

const (
	JobQueueIdColumn              = "id"
	JobQueueNodeIdColumn          = "node_id"
	JobQueueLowerBoundJobId       = "lower_bound_job_id"
	JobQueueUpperBound            = "upper_bound_job_id"
	JobQueueVersion               = "version"
	JobNumberOfActiveNodesVersion = "number_of_active_nodes"
	JobQueueDateCreatedColumn     = "date_created"
)

type jobQueues struct {
	fsmStore              fsm.Scheduler0RaftStore
	logger                hclog.Logger
	scheduler0RaftActions fsm.Scheduler0RaftActions
}

//go:generate mockery --name JobQueuesRepo --output ./ --inpackage
type JobQueuesRepo interface {
	GetLastJobQueueLogForNode(nodeId uint64, version uint64) []models.JobQueueLog
	IncrementQueueVersion(numberOfServer int)
	GetLastVersion() uint64
	InsertJobQueueLogs(logs []models.JobQueueLog)
	GetJobQueueByLastInsertedAndRowsAffected(lastInsertedId, rowsAffected int64) []models.JobQueueLog
}

func NewJobQueuesRepo(logger hclog.Logger, scheduler0RaftActions fsm.Scheduler0RaftActions, store fsm.Scheduler0RaftStore) *jobQueues {
	return &jobQueues{
		logger:                logger.Named("job-queue-repo"),
		fsmStore:              store,
		scheduler0RaftActions: scheduler0RaftActions,
	}
}

func (repo *jobQueues) GetLastJobQueueLogForNode(nodeId uint64, version uint64) []models.JobQueueLog {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	var result []models.JobQueueLog

	selectBuilder := sq.Select(
		JobQueueIdColumn,
		JobQueueNodeIdColumn,

		JobQueueLowerBoundJobId,
		JobQueueUpperBound,
		JobQueueVersion,
		JobQueueDateCreatedColumn,
	).
		From(JobQueuesTableName).
		Where(fmt.Sprintf("%s = ? AND %s = ?", JobQueueNodeIdColumn, JobQueueVersion), nodeId, version).
		OrderBy(fmt.Sprintf("%s DESC", JobQueueDateCreatedColumn)).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	defer rows.Close()
	if err != nil {
		repo.logger.Error("failed to build query to fetch queue logs", err)
		return nil
	}
	for rows.Next() {
		queueLog := models.JobQueueLog{}
		scanErr := rows.Scan(
			&queueLog.Id,
			&queueLog.NodeId,
			&queueLog.LowerBoundJobId,
			&queueLog.UpperBoundJobId,
			&queueLog.Version,
			&queueLog.DateCreated,
		)
		if scanErr != nil {
			repo.logger.Error("scan error fetching queue log", scanErr)
			return nil
		}
		result = append(result, queueLog)
	}
	if rows.Err() != nil {
		repo.logger.Error("rows error fetching queue log", err)
		return nil
	}

	return result
}

func (repo *jobQueues) IncrementQueueVersion(numberOfServer int) {
	lastVersion := repo.getLastVersion()
	_, err := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(),
		constants.CommandTypeDbExecute,
		fmt.Sprintf("insert into %s (%s, %s) values (?, ?)",
			JobQueuesVersionTableName,
			JobQueueVersion,
			JobNumberOfActiveNodesVersion,
		), []interface{}{lastVersion + 1, numberOfServer}, nil, 0)
	if err != nil {
		log.Fatalln("failed to increment job queue version", err)
	}
}

func (repo *jobQueues) InsertJobQueueLogs(logs []models.JobQueueLog) {
	batches := utils.Batch[models.JobQueueLog](logs, 5)

	schedulerTime := scheduler0time.GetSchedulerTime()
	now := schedulerTime.GetTime(time.Now())

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO job_queues (%s, %s, %s, %s, %s) VALUES ",
			constants.JobQueueNodeIdColumn,
			constants.JobQueueLowerBoundJobId,
			constants.JobQueueUpperBound,
			constants.JobQueueVersion,
			constants.JobQueueDateCreatedColumn,
		)
		params := []interface{}{}
		nodeIds := []uint64{}

		for i, job := range batch {
			query += fmt.Sprint("(?, ?, ?, ?, ?)")
			job.DateCreated = now
			params = append(params,
				job.NodeId,
				job.LowerBoundJobId,
				job.UpperBoundJobId,
				job.Version,
				job.DateCreated,
			)

			nodeIds = append(nodeIds, job.NodeId)
			if i < len(batch)-1 {
				query += ","
			}
		}

		query += ";"

		_, applyErr := repo.scheduler0RaftActions.WriteCommandToRaftLog(
			repo.fsmStore.GetRaft(),
			constants.CommandTypeDbExecute,
			query,
			params,
			nodeIds,
			constants.CommandActionQueueJob,
		)
		if applyErr != nil {
			if applyErr == raft.ErrNotLeader {
				repo.logger.Error("failed to insert job queue: logs raft leader not found")
			}
		}
	}
}

func (repo *jobQueues) GetJobQueueByLastInsertedAndRowsAffected(lastInsertedId, rowsAffected int64) []models.JobQueueLog {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	var result []models.JobQueueLog

	selectBuilder := sq.Select(
		JobQueueIdColumn,
		JobQueueNodeIdColumn,

		JobQueueLowerBoundJobId,
		JobQueueUpperBound,
		JobQueueVersion,
		JobQueueDateCreatedColumn,
	).
		From(JobQueuesTableName).
		Where(fmt.Sprintf("%s BETWEEN ? AND ?", JobQueueIdColumn), lastInsertedId-rowsAffected, lastInsertedId).
		OrderBy(fmt.Sprintf("%s DESC", JobQueueDateCreatedColumn)).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	defer rows.Close()
	if err != nil {
		repo.logger.Error("failed to build query to fetch queue logs", "error", err)
		return result
	}
	for rows.Next() {
		queueLog := models.JobQueueLog{}
		scanErr := rows.Scan(
			&queueLog.Id,
			&queueLog.NodeId,
			&queueLog.LowerBoundJobId,
			&queueLog.UpperBoundJobId,
			&queueLog.Version,
			&queueLog.DateCreated,
		)
		if scanErr != nil {
			repo.logger.Error("scan error fetching queue log", scanErr)
			return nil
		}
		result = append(result, queueLog)
	}
	if rows.Err() != nil {
		repo.logger.Error("rows error fetching queue log", err)
		return result
	}

	return result
}

func (repo *jobQueues) GetLastVersion() uint64 {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()
	return repo.getLastVersion()
}

func (repo *jobQueues) getLastVersion() uint64 {
	selectBuilder := sq.Select(fmt.Sprintf("MAX(%s)", JobQueueVersion)).
		From(JobQueuesVersionTableName).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	defer rows.Close()
	if err != nil {
		log.Fatal("failed to build query to fetch queue logs", err)
	}
	var version *uint64
	for rows.Next() {
		scanErr := rows.Scan(&version)
		if scanErr != nil {
			log.Fatal("scan error fetching last queue version", scanErr)
		}
	}
	if rows.Err() != nil {
		log.Fatal("rows error fetching queue log", err)
	}

	if version == nil {
		return 0
	}

	return *version
}
