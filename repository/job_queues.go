package repository

import (
	"fmt"
	sq "github.com/Masterminds/squirrel"
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
	fsmStore *fsm.Store
	logger   *log.Logger
}

type JobQueuesRepo interface {
	GetLastJobQueueLogForNode(nodeId int, version uint64) []models.JobQueueLog
	GetLastVersion() uint64
}

func NewJobQueuesRepo(logger *log.Logger, store *fsm.Store) *jobQueues {
	return &jobQueues{
		logger:   logger,
		fsmStore: store,
	}
}

func (repo *jobQueues) GetLastJobQueueLogForNode(nodeId int, version uint64) []models.JobQueueLog {
	repo.fsmStore.DataStore.ConnectionLock.Lock()
	defer repo.fsmStore.DataStore.ConnectionLock.Unlock()

	result := []models.JobQueueLog{}

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
		RunWith(repo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Fatalln("failed to build query to fetch queue logs", err)
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
			repo.logger.Fatalln("scan error fetching queue log", scanErr)
		}
		result = append(result, queueLog)
	}
	if rows.Err() != nil {
		repo.logger.Fatalln("rows error fetching queue log", err)
	}

	return result
}

func (repo *jobQueues) GetLastVersion() uint64 {
	repo.fsmStore.DataStore.ConnectionLock.Lock()
	defer repo.fsmStore.DataStore.ConnectionLock.Unlock()

	selectBuilder := sq.Select(fmt.Sprintf("MAX(%s)", JobQueueVersion)).
		From(JobQueuesVersionTableName).
		RunWith(repo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Fatalln("failed to build query to fetch queue logs", err)
	}
	var version *uint64
	for rows.Next() {
		scanErr := rows.Scan(&version)
		if scanErr != nil {
			repo.logger.Fatalln("scan error fetching last queue version", scanErr)
		}
	}
	if rows.Err() != nil {
		repo.logger.Fatalln("rows error fetching queue log", err)
	}

	if version == nil {
		return 0
	}

	return *version
}
