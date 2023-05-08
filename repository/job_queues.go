package repository

import (
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"log"
	"scheduler0/fsm"
	"scheduler0/models"
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

type JobQueuesRepo interface {
	GetLastJobQueueLogForNode(nodeId uint64, version uint64) []models.JobQueueLog
	GetLastVersion() uint64
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

func (repo *jobQueues) GetLastVersion() uint64 {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

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
