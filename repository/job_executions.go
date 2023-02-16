package repository

import (
	"context"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"github.com/robfig/cron"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/models"
	"scheduler0/utils"
	"scheduler0/utils/batcher"
	"time"
)

const (
	ExecutionsCommittedTableName   = "job_executions_committed"
	ExecutionsUnCommittedTableName = "job_executions_uncommitted"
)

const (
	ExecutionsIdColumn                = "id"
	ExecutionsUniqueIdColumn          = "unique_id"
	ExecutionsStateColumn             = "state"
	ExecutionsNodeIdColumn            = "node_id"
	ExecutionsLastExecutionTimeColumn = "last_execution_time"
	ExecutionsNextExecutionTime       = "next_execution_time"
	ExecutionsJobQueueVersion         = "job_queue_version"
	ExecutionsJobIdColumn             = "job_id"
	ExecutionsDateCreatedColumn       = "date_created"
	ExecutionsVersion                 = "execution_version"
)

type ExecutionsRepo interface {
	BatchInsert(jobs []models.JobModel, nodeId uint64, state models.JobExecutionLogState, jobQueueVersion uint64, executionVersions map[uint64]uint64)
	CountLastFailedExecutionLogs(jobId uint64, nodeId uint64, executionVersion uint64) uint64
	CountExecutionLogs(committed bool) uint64
	GetUncommittedExecutionsLogForNode(nodeId uint64) []models.JobExecutionLog
	GetLastExecutionLogForJobIds(jobIds []uint64) map[uint64]models.JobExecutionLog
}

type executionsRepo struct {
	fsmStore *fsm.Store
	logger   hclog.Logger
}

func NewExecutionsRepo(logger hclog.Logger, store *fsm.Store) *executionsRepo {
	return &executionsRepo{
		fsmStore: store,
		logger:   logger.Named("job-executions-repo"),
	}
}

func (repo *executionsRepo) BatchInsert(jobs []models.JobModel, nodeId uint64, state models.JobExecutionLogState, jobQueueVersion uint64, jobExecutionVersions map[uint64]uint64) {
	repo.fsmStore.DataStore.ConnectionLock.Lock()
	defer repo.fsmStore.DataStore.ConnectionLock.Unlock()

	if len(jobs) < 1 {
		return
	}

	batches := batcher.Batch[models.JobModel](jobs, 9)
	returningIds := []uint64{}

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s , %s, %s) VALUES ",
			ExecutionsUnCommittedTableName,
			ExecutionsUniqueIdColumn,
			ExecutionsStateColumn,
			ExecutionsNodeIdColumn,
			ExecutionsLastExecutionTimeColumn,
			ExecutionsNextExecutionTime,
			ExecutionsJobIdColumn,
			ExecutionsJobQueueVersion,
			ExecutionsDateCreatedColumn,
			ExecutionsVersion,
		)
		params := []interface{}{}
		ids := []uint64{}

		for i, job := range batch {
			executionVersion := 0

			if jobExecutionVersion, ok := jobExecutionVersions[uint64(job.ID)]; ok {
				executionVersion = int(jobExecutionVersion)
			}

			query += fmt.Sprint("(?, ?, ?, ?, ?, ?, ?, ?, ?)")
			schedule, parseErr := cron.Parse(job.Spec)
			if parseErr != nil {
				repo.logger.Error(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
			}
			executionTime := schedule.Next(jobs[i].LastExecutionDate)
			schedulerTime := utils.GetSchedulerTime()
			now := schedulerTime.GetTime(time.Now())
			params = append(params,
				job.ExecutionId,
				state,
				nodeId,
				job.LastExecutionDate,
				executionTime,
				job.ID,
				jobQueueVersion,
				now,
				executionVersion,
			)
			if i < len(batch)-1 {
				query += ","
			}
		}

		query += ";"
		ctx := context.Background()
		tx, err := repo.fsmStore.DataStore.Connection.BeginTx(ctx, nil)
		if err != nil {
			repo.logger.Error("failed to create transaction for batch insertion", err)
			return
		}

		res, err := tx.Exec(query, params...)
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				repo.logger.Error("failed to rollback failed batch insertion execute ", err)
				return
			} else {
				repo.logger.Error("failed to execute batch insertion", err)
				return
			}
		}
		err = tx.Commit()
		if err != nil {
			repo.logger.Error("failed to commit execute batch insertion", err)
			return
		}

		lastInsertedId, err := res.LastInsertId()
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				repo.logger.Error("failed to rollback, failed batch insertion execute, failed to get last inserted id", err)
				return
			} else {
				repo.logger.Error("failed to execute batch insertion, failed to get last inserted id", err)
				return
			}
		}

		for i := lastInsertedId - int64(len(batch)) + 1; i <= lastInsertedId; i++ {
			ids = append(ids, uint64(i))
		}

		returningIds = append(returningIds, ids...)
	}
}

func (repo *executionsRepo) getLastExecutionLogForJobIds(jobIds []uint64) []models.JobExecutionLog {
	repo.fsmStore.DataStore.ConnectionLock.Lock()
	defer repo.fsmStore.DataStore.ConnectionLock.Unlock()

	results := []models.JobExecutionLog{}

	if len(jobIds) < 1 {
		return results
	}

	batches := batcher.Batch[uint64](jobIds, 1)

	for _, batch := range batches {
		paramsPlaceholder := "?"
		params := []interface{}{batch[0]}

		for _, jobId := range batch[1:] {
			paramsPlaceholder += ",?"
			params = append(params, jobId)
		}

		query := fmt.Sprintf(
			"select %s, %s, %s, %s, %s, %s, %s, %s, %s, %s from (select %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, row_number() over (partition by job_id order by execution_version desc, state desc) rowNum from (select %s, %s, %s, %s, %s, %s, %s, %s, %s, %s from job_executions_committed union all select %s, %s, %s, %s, %s, %s, %s, %s, %s, %s from job_executions_uncommitted) where %s in (%s)) t where t.rowNum = 1",
			ExecutionsVersion,
			ExecutionsStateColumn,
			ExecutionsIdColumn,
			ExecutionsUniqueIdColumn,
			ExecutionsNodeIdColumn,
			ExecutionsLastExecutionTimeColumn,
			ExecutionsNextExecutionTime,
			ExecutionsJobIdColumn,
			ExecutionsDateCreatedColumn,
			ExecutionsJobQueueVersion,
			ExecutionsVersion,
			ExecutionsStateColumn,
			ExecutionsIdColumn,
			ExecutionsUniqueIdColumn,
			ExecutionsNodeIdColumn,
			ExecutionsLastExecutionTimeColumn,
			ExecutionsNextExecutionTime,
			ExecutionsJobIdColumn,
			ExecutionsDateCreatedColumn,
			ExecutionsJobQueueVersion,
			ExecutionsVersion,
			ExecutionsStateColumn,
			ExecutionsIdColumn,
			ExecutionsUniqueIdColumn,
			ExecutionsNodeIdColumn,
			ExecutionsLastExecutionTimeColumn,
			ExecutionsNextExecutionTime,
			ExecutionsJobIdColumn,
			ExecutionsDateCreatedColumn,
			ExecutionsJobQueueVersion,
			ExecutionsVersion,
			ExecutionsStateColumn,
			ExecutionsIdColumn,
			ExecutionsUniqueIdColumn,
			ExecutionsNodeIdColumn,
			ExecutionsLastExecutionTimeColumn,
			ExecutionsNextExecutionTime,
			ExecutionsJobIdColumn,
			ExecutionsDateCreatedColumn,
			ExecutionsJobQueueVersion,
			ExecutionsJobIdColumn,
			paramsPlaceholder,
		)

		rows, err := repo.fsmStore.DataStore.Connection.Query(query, params...)
		if err != nil {
			repo.logger.Error("failed to select last execution log", err)
			return nil
		}
		for rows.Next() {
			lastExecutionLog := models.JobExecutionLog{}
			scanErr := rows.Scan(
				&lastExecutionLog.ExecutionVersion,
				&lastExecutionLog.State,
				&lastExecutionLog.Id,
				&lastExecutionLog.UniqueId,
				&lastExecutionLog.NodeId,
				&lastExecutionLog.LastExecutionDatetime,
				&lastExecutionLog.NextExecutionDatetime,
				&lastExecutionLog.JobId,
				&lastExecutionLog.DataCreated,
				&lastExecutionLog.JobQueueVersion,
			)
			if err != nil {
				repo.logger.Error("failed to scan rows", scanErr)
				return nil
			}
			results = append(results, lastExecutionLog)
		}
		if rows.Err() != nil {
			repo.logger.Error("failed to select last execution log rows error", rows.Err())
			return nil
		}
	}

	return results
}

func (repo *executionsRepo) GetLastExecutionLogForJobIds(jobIds []uint64) map[uint64]models.JobExecutionLog {
	lastCommittedExecutionLogs := repo.getLastExecutionLogForJobIds(jobIds)

	executionLogsMap := map[uint64]models.JobExecutionLog{}

	for _, jobId := range jobIds {
		for _, lastCommittedExecutionLog := range lastCommittedExecutionLogs {
			if uint64(lastCommittedExecutionLog.JobId) == jobId {
				if lastKnownExecutionLog, ok := executionLogsMap[jobId]; !ok {
					executionLogsMap[jobId] = lastCommittedExecutionLog
				} else {
					if lastKnownExecutionLog.ExecutionVersion < lastCommittedExecutionLog.ExecutionVersion {
						executionLogsMap[jobId] = lastCommittedExecutionLog
					} else {
						if lastKnownExecutionLog.State < lastCommittedExecutionLog.State {
							executionLogsMap[uint64(int64(jobId))] = lastCommittedExecutionLog
						}
					}
				}

			}
		}
	}

	return executionLogsMap
}

func (repo *executionsRepo) CountLastFailedExecutionLogs(jobId uint64, nodeId uint64, executionVersion uint64) uint64 {
	repo.fsmStore.DataStore.ConnectionLock.Lock()
	defer repo.fsmStore.DataStore.ConnectionLock.Unlock()

	query := fmt.Sprintf("select count(*) from ("+
		"select * from job_executions_committed union all select * from job_executions_uncommitted"+
		") where %s = ? AND %s = ? AND %s = ? AND %s = ? order by %s desc limit 1",
		ExecutionsJobIdColumn,
		ExecutionsVersion,
		ExecutionsNodeIdColumn,
		ExecutionsStateColumn,
		ExecutionsVersion,
	)

	rows, err := repo.fsmStore.DataStore.Connection.Query(query, jobId, executionVersion, nodeId, models.ExecutionLogFailedState)
	if err != nil {
		repo.logger.Error("failed to select last execution log", err)
		return 0
	}
	var count uint64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if err != nil {
			repo.logger.Error("failed to scan rows ", scanErr)
			return 0
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("failed to select last execution log rows error", rows.Err())
		return 0
	}
	return count
}

func (repo *executionsRepo) CountExecutionLogs(committed bool) uint64 {
	repo.fsmStore.DataStore.ConnectionLock.Lock()
	defer repo.fsmStore.DataStore.ConnectionLock.Unlock()

	tableName := ExecutionsUnCommittedTableName

	if committed {
		tableName = ExecutionsCommittedTableName
	}

	selectBuilder := sq.Select("count(*)").
		From(tableName).
		RunWith(repo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("failed to select last execution log", err)
		return 0
	}
	var count uint64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if err != nil {
			repo.logger.Error("failed to scan rows ", scanErr)
			return 0
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("failed to select last execution log rows error", rows.Err())
		return 0
	}
	return count
}

func (repo *executionsRepo) GetUncommittedExecutionsLogForNode(nodeId uint64) []models.JobExecutionLog {
	repo.fsmStore.DataStore.ConnectionLock.Lock()
	defer repo.fsmStore.DataStore.ConnectionLock.Unlock()

	selectBuilder := sq.Select(
		ExecutionsIdColumn,
		ExecutionsUniqueIdColumn,
		ExecutionsStateColumn,
		ExecutionsNodeIdColumn,
		ExecutionsLastExecutionTimeColumn,
		ExecutionsNextExecutionTime,
		ExecutionsJobIdColumn,
		ExecutionsDateCreatedColumn,
		ExecutionsJobQueueVersion,
		ExecutionsVersion,
	).
		From(ExecutionsUnCommittedTableName).
		OrderBy(fmt.Sprintf("%s DESC", ExecutionsNextExecutionTime)).
		Where(fmt.Sprintf("%s = ?", ExecutionsNodeIdColumn), nodeId).
		Limit(constants.JobExecutionLogMaxBatchSize).
		RunWith(repo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("failed to select last execution log", err)
		return nil
	}
	results := []models.JobExecutionLog{}
	for rows.Next() {
		lastExecutionLog := models.JobExecutionLog{}
		scanErr := rows.Scan(
			&lastExecutionLog.Id,
			&lastExecutionLog.UniqueId,
			&lastExecutionLog.State,
			&lastExecutionLog.NodeId,
			&lastExecutionLog.LastExecutionDatetime,
			&lastExecutionLog.NextExecutionDatetime,
			&lastExecutionLog.JobId,
			&lastExecutionLog.DataCreated,
			&lastExecutionLog.JobQueueVersion,
			&lastExecutionLog.ExecutionVersion,
		)
		if scanErr != nil {
			repo.logger.Error("failed to scan rows", scanErr)
			return nil
		}
		results = append(results, lastExecutionLog)
	}
	if rows.Err() != nil {
		repo.logger.Error("failed to select last execution log rows error", rows.Err())
		return nil
	}

	return results
}