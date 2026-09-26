package job_queue

import (
	"fmt"
	"log"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/utils"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
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
	IncrementQueueVersion(numberOfServer int)
	GetLastVersion() uint64
	InsertJobQueueLogs(logs []models.JobQueueLog)
	GetJobQueueByLastInsertedAndRowsAffected(lastInsertedId, rowsAffected int64) []models.JobQueueLog
	// GetMostRecentJobQueueDate returns the date_created of the most recently inserted job queue log,
	// or a zero time if no job queues exist. Used for quota reconciliation.
	GetMostRecentJobQueueDate() (time.Time, error)
	GetAllJobQueues() ([]models.JobQueueLog, error)
	GetAllJobQueueVersions() ([]models.JobQueueVersion, error)
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
		repo.logger.Error("GetLastJobQueueLogForNode: failed to build query to fetch queue logs", "error", err, "nodeId", nodeId, "version", version)
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
			repo.logger.Error("GetLastJobQueueLogForNode: scan error fetching queue log", "error", scanErr, "nodeId", nodeId, "version", version)
			return nil
		}
		result = append(result, queueLog)
	}
	if rows.Err() != nil {
		repo.logger.Error("GetLastJobQueueLogForNode: rows error fetching queue log", "error", rows.Err(), "nodeId", nodeId, "version", version)
		return nil
	}

	return result
}

func (repo *jobQueues) IncrementQueueVersion(numberOfServer int) {
	lastVersion := repo.getLastVersion()
	_, err := repo.scheduler0RaftActions.WriteCommandToRaftLog(
		repo.fsmStore.GetRaft(),
		constants.CommandTypeDbExecute,
		fmt.Sprintf("insert into %s (%s, %s, %s) values (?, ?, ?)",
			JobQueuesVersionTableName,
			JobQueueVersion,
			JobNumberOfActiveNodesVersion,
			JobQueueDateCreatedColumn,
		), []interface{}{lastVersion + 1, numberOfServer, scheduler0time.GetSchedulerTime().GetTime(time.Now())}, nil, 0)
	if err != nil {
		repo.logger.Error("IncrementQueueVersion: failed to increment job queue version", "error", err, "numberOfServer", numberOfServer, "lastVersion", lastVersion)
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
				repo.logger.Error("InsertJobQueueLogs: failed to insert job queue logs raft leader not found", "error", applyErr, "batchSize", len(batch))
			} else {
				repo.logger.Error("InsertJobQueueLogs: failed to insert job queue logs", "error", applyErr, "batchSize", len(batch))
			}
		}
	}
}

func (repo *jobQueues) GetJobQueueByLastInsertedAndRowsAffected(lastInsertedId, rowsAffected int64) []models.JobQueueLog {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	var result []models.JobQueueLog

	// Build list of IDs from lastInsertedId to lastInsertedId + rowsAffected - 1
	paramsPlaceholder := ""
	ids := []interface{}{}

	for i := int64(0); i < rowsAffected; i++ {
		paramsPlaceholder += "?"
		if i < rowsAffected-1 {
			paramsPlaceholder += ","
		}
		ids = append(ids, lastInsertedId+i)
	}

	selectBuilder := sq.Select(
		JobQueueIdColumn,
		JobQueueNodeIdColumn,

		JobQueueLowerBoundJobId,
		JobQueueUpperBound,
		JobQueueVersion,
		JobQueueDateCreatedColumn,
	).
		From(JobQueuesTableName).
		Where(fmt.Sprintf("%s IN (%s)", JobQueueIdColumn, paramsPlaceholder), ids...).
		OrderBy(fmt.Sprintf("%s DESC", JobQueueDateCreatedColumn)).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	defer rows.Close()
	if err != nil {
		repo.logger.Error("GetJobQueueByLastInsertedAndRowsAffected: failed to build query to fetch queue logs", "error", err, "lastInsertedId", lastInsertedId, "rowsAffected", rowsAffected)
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
			repo.logger.Error("GetJobQueueByLastInsertedAndRowsAffected: scan error fetching queue log", "error", scanErr, "lastInsertedId", lastInsertedId, "rowsAffected", rowsAffected)
			return nil
		}
		result = append(result, queueLog)
	}
	if rows.Err() != nil {
		repo.logger.Error("GetJobQueueByLastInsertedAndRowsAffected: rows error fetching queue log", "error", rows.Err(), "lastInsertedId", lastInsertedId, "rowsAffected", rowsAffected)
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
		repo.logger.Error("getLastVersion: failed to build query to fetch queue logs", "error", err)
		log.Fatal("failed to build query to fetch queue logs", err)
	}
	var version *uint64
	for rows.Next() {
		scanErr := rows.Scan(&version)
		if scanErr != nil {
			repo.logger.Error("getLastVersion: scan error fetching last queue version", "error", scanErr)
			log.Fatal("scan error fetching last queue version", scanErr)
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("getLastVersion: rows error fetching queue log", "error", rows.Err())
		log.Fatal("rows error fetching queue log", rows.Err())
	}

	if version == nil {
		return 0
	}

	return *version
}

func (repo *jobQueues) GetMostRecentJobQueueDate() (time.Time, error) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	// Get the last version number
	lastVersion := repo.getLastVersion()
	if lastVersion == 0 {
		// No job queue versions exist yet, return zero time
		return time.Time{}, nil
	}

	// Query the job_queue_versions table for the date_created of the last version
	selectBuilder := sq.Select(JobQueueDateCreatedColumn).
		From(JobQueuesVersionTableName).
		Where(fmt.Sprintf("%s = ?", JobQueueVersion), lastVersion).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("GetMostRecentJobQueueDate: failed to query job queue version date", "error", err, "version", lastVersion)
		return time.Time{}, err
	}
	defer rows.Close()

	var dateCreated time.Time
	found := false
	for rows.Next() {
		scanErr := rows.Scan(&dateCreated)
		if scanErr != nil {
			repo.logger.Error("GetMostRecentJobQueueDate: failed to scan row", "error", scanErr, "version", lastVersion)
			return time.Time{}, scanErr
		}
		found = true
	}

	if rows.Err() != nil {
		repo.logger.Error("GetMostRecentJobQueueDate: rows error", "error", rows.Err(), "version", lastVersion)
		return time.Time{}, rows.Err()
	}

	if !found {
		// Version exists but no date found (shouldn't happen, but handle gracefully)
		repo.logger.Warn("GetMostRecentJobQueueDate: version found but no date_created", "version", lastVersion)
		return time.Time{}, nil
	}

	return dateCreated, nil
}

func (repo *jobQueues) GetAllJobQueues() ([]models.JobQueueLog, error) {
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
		OrderBy(fmt.Sprintf("%s ASC", JobQueueIdColumn)).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("GetAllJobQueues: failed to build query to fetch all job queues", "error", err)
		return nil, err
	}
	defer rows.Close()

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
			repo.logger.Error("GetAllJobQueues: scan error fetching job queue log", "error", scanErr)
			return nil, scanErr
		}
		result = append(result, queueLog)
	}

	if rows.Err() != nil {
		repo.logger.Error("GetAllJobQueues: rows error fetching job queue logs", "error", rows.Err())
		return nil, rows.Err()
	}

	return result, nil
}

func (repo *jobQueues) GetAllJobQueueVersions() ([]models.JobQueueVersion, error) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	var result []models.JobQueueVersion

	selectBuilder := sq.Select(
		"id",
		JobQueueVersion,
		JobNumberOfActiveNodesVersion,
		JobQueueDateCreatedColumn,
	).
		From(JobQueuesVersionTableName).
		OrderBy(fmt.Sprintf("%s ASC", JobQueueVersion)).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("GetAllJobQueueVersions: failed to build query to fetch all job queue versions", "error", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		version := models.JobQueueVersion{}
		scanErr := rows.Scan(
			&version.Id,
			&version.Version,
			&version.NumberOfActiveNodes,
			&version.DateCreated,
		)
		if scanErr != nil {
			repo.logger.Error("GetAllJobQueueVersions: scan error fetching job queue version", "error", scanErr)
			return nil, scanErr
		}
		result = append(result, version)
	}

	if rows.Err() != nil {
		repo.logger.Error("GetAllJobQueueVersions: rows error fetching job queue versions", "error", rows.Err())
		return nil, rows.Err()
	}

	return result, nil
}
