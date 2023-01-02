package repository

import (
	"context"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/araddon/dateparse"
	"github.com/robfig/cron"
	"log"
	"scheduler0/constants"
	"scheduler0/fsm"
	"scheduler0/models"
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
	BatchInsert(jobs []models.JobModel, nodeId uint64, state models.JobExecutionLogState, jobQueueVersion uint64, executionVersions map[int64]uint64)
	CountLastFailedExecutionLogs(jobId int64, nodeId int, committed bool) uint64
	CountExecutionLogs(committed bool) uint64
	GetUncommittedExecutionsLogForNode(nodeId int) []models.JobExecutionLog
	GetLastExecutionLogForJobIds(jobIds []int64) map[int64]models.JobExecutionLog
	GetLastExecutionVersionForJobIds(jobIds []int64, committed bool) map[int64]uint64
}

type executionsRepo struct {
	fsmStore *fsm.Store
	logger   *log.Logger
}

func NewExecutionsRepo(logger *log.Logger, store *fsm.Store) *executionsRepo {
	return &executionsRepo{
		fsmStore: store,
		logger:   logger,
	}
}

func (repo *executionsRepo) BatchInsert(jobs []models.JobModel, nodeId uint64, state models.JobExecutionLogState, jobQueueVersion uint64, jobExecutionVersions map[int64]uint64) {
	repo.fsmStore.DataStore.ConnectionLock.Lock()
	defer repo.fsmStore.DataStore.ConnectionLock.Unlock()

	if len(jobs) < 1 {
		return
	}

	batches := make([][]models.JobModel, 0)

	if len(jobs) > constants.JobExecutionLogMaxBatchSize {
		temp := make([]models.JobModel, 0)
		count := 0
		for count < len(jobs) {
			temp = append(temp, jobs[count])
			if len(temp) == constants.JobExecutionLogMaxBatchSize {
				batches = append(batches, temp)
				temp = make([]models.JobModel, 0)
			}
			count += 1
		}
		if len(temp) > 0 {
			batches = append(batches, temp)
			temp = make([]models.JobModel, 0)
		}
	} else {
		batches = append(batches, jobs)
	}

	returningIds := []int64{}

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
		ids := []int64{}

		for i, job := range batch {
			executionVersion := 0

			if jobExecutionVersion, ok := jobExecutionVersions[job.ID]; ok {
				executionVersion = int(jobExecutionVersion)
			}

			query += fmt.Sprint("(?, ?, ?, ?, ?, ?, ?, ?, ?)")
			schedule, parseErr := cron.Parse(job.Spec)
			if parseErr != nil {
				repo.logger.Fatalln(fmt.Sprintf("failed to parse job cron spec %s", parseErr.Error()))
			}
			executionTime := schedule.Next(jobs[i].LastExecutionDate)
			params = append(params,
				job.ExecutionId,
				state,
				nodeId,
				job.LastExecutionDate,
				executionTime,
				job.ID,
				jobQueueVersion,
				time.Now().UTC(),
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
			repo.logger.Fatalln("failed to create transaction for batch insertion", err)
		}

		res, err := tx.Exec(query, params...)
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				repo.logger.Fatalln("failed to rollback failed batch insertion execute ", err)
			} else {
				repo.logger.Fatalln("failed to execute batch insertion", err)
			}
		}
		err = tx.Commit()
		if err != nil {
			repo.logger.Fatalln("failed to commit execute batch insertion", err)
		}

		lastInsertedId, err := res.LastInsertId()
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				repo.logger.Fatalln("failed to rollback, failed batch insertion execute, failed to get last inserted id", err)
			} else {
				repo.logger.Fatalln("failed to execute batch insertion, failed to get last inserted id", err)
			}
		}

		for i := lastInsertedId - int64(len(batch)) + 1; i <= lastInsertedId; i++ {
			ids = append(ids, i)
		}

		returningIds = append(returningIds, ids...)
	}
}

func (repo *executionsRepo) getLastExecutionLogForJobIds(jobIds []int64) []models.JobExecutionLog {
	repo.fsmStore.DataStore.ConnectionLock.Lock()
	defer repo.fsmStore.DataStore.ConnectionLock.Unlock()

	results := []models.JobExecutionLog{}

	if len(jobIds) < 1 {
		return results
	}

	paramsPlaceholder := "?"
	params := []interface{}{jobIds[0]}

	for _, jobId := range jobIds[1:] {
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
		repo.logger.Fatalln("failed to select last execution log", err)
	}
	for rows.Next() {
		lastExecutionLog := models.JobExecutionLog{}
		var dateCreatedStr string
		var nextExecutionDTStr string
		var lastExecutionDTStr string
		err := rows.Scan(
			&lastExecutionLog.ExecutionVersion,
			&lastExecutionLog.State,
			&lastExecutionLog.Id,
			&lastExecutionLog.UniqueId,
			&lastExecutionLog.NodeId,
			&lastExecutionDTStr,
			&nextExecutionDTStr,
			&lastExecutionLog.JobId,
			&dateCreatedStr,
			&lastExecutionLog.JobQueueVersion,
		)
		t, errParse := dateparse.ParseLocal(dateCreatedStr)
		if errParse != nil {
			repo.logger.Fatalln("failed to parse date created string", errParse)
		}
		lastExecutionLog.DataCreated = t
		t, errParse = dateparse.ParseLocal(nextExecutionDTStr)
		if errParse != nil {
			repo.logger.Fatalln("failed to parse next execution date time string", errParse)
		}
		lastExecutionLog.NextExecutionDatetime = t
		t, errParse = dateparse.ParseLocal(lastExecutionDTStr)
		if errParse != nil {
			repo.logger.Fatalln("failed to last execution date time string", errParse)
		}
		lastExecutionLog.LastExecutionDatetime = t
		if err != nil {
			repo.logger.Fatalln("failed to scan rows", errParse)
		}
		results = append(results, lastExecutionLog)
	}
	if rows.Err() != nil {
		repo.logger.Fatalln("failed to select last execution log rows error", rows.Err())
	}

	return results
}

func (repo *executionsRepo) GetLastExecutionLogForJobIds(jobIds []int64) map[int64]models.JobExecutionLog {
	lastCommittedExecutionLogs := repo.getLastExecutionLogForJobIds(jobIds)

	executionLogsMap := map[int64]models.JobExecutionLog{}

	for _, jobId := range jobIds {
		for _, lastCommittedExecutionLog := range lastCommittedExecutionLogs {
			if lastCommittedExecutionLog.JobId == jobId {
				if lastKnownExecutionLog, ok := executionLogsMap[jobId]; !ok {
					executionLogsMap[jobId] = lastCommittedExecutionLog
				} else {
					if lastKnownExecutionLog.ExecutionVersion < lastCommittedExecutionLog.ExecutionVersion {
						executionLogsMap[jobId] = lastCommittedExecutionLog
					} else {
						if lastKnownExecutionLog.State < lastCommittedExecutionLog.State {
							executionLogsMap[jobId] = lastCommittedExecutionLog
						}
					}
				}

			}
		}
	}

	return executionLogsMap
}

func (repo *executionsRepo) GetLastExecutionVersionForJobIds(jobIds []int64, committed bool) map[int64]uint64 {
	repo.fsmStore.DataStore.ConnectionLock.Lock()
	defer repo.fsmStore.DataStore.ConnectionLock.Unlock()

	results := map[int64]uint64{}

	if len(jobIds) < 1 {
		return results
	}

	cachedJobId := map[int64]int64{}

	paramsPlaceholder := "?"
	predicateIds := []interface{}{jobIds[0]}

	for _, jobId := range jobIds[1:] {
		if _, ok := cachedJobId[jobId]; !ok {
			cachedJobId[jobId] = jobId
			paramsPlaceholder += ",?"
		}
	}

	for _, jobId := range cachedJobId {
		predicateIds = append(predicateIds, jobId)
	}

	tableName := ExecutionsUnCommittedTableName

	if committed {
		tableName = ExecutionsCommittedTableName
	}

	query := fmt.Sprintf(
		"select %s, %s, %s from (select %s, %s, %s, row_number() over (partition by job_id order by execution_version desc, state desc) rowNum from %s where %s in (%s)) t where t.rowNum = 1",
		ExecutionsVersion,
		ExecutionsStateColumn,
		ExecutionsJobIdColumn,
		ExecutionsVersion,
		ExecutionsStateColumn,
		ExecutionsJobIdColumn,
		tableName,
		ExecutionsJobIdColumn,
		paramsPlaceholder,
	)

	rows, err := repo.fsmStore.DataStore.Connection.Query(query, predicateIds...)
	if err != nil {
		repo.logger.Fatalln("failed to select last execution version", err)
	}
	for rows.Next() {
		var version uint64 = 0
		var state uint64 = 0
		var jobId int64 = 0
		scanErr := rows.Scan(&version, &state, &jobId)
		if scanErr != nil {
			repo.logger.Fatalln("failed to select last execution version, scan error", scanErr)
		}
		results[jobId] = version
	}
	if rows.Err() != nil {
		repo.logger.Fatalln("failed to select last execution version rows error", rows.Err())
	}

	return results
}

func (repo *executionsRepo) CountLastFailedExecutionLogs(jobId int64, nodeId int, committed bool) uint64 {
	repo.fsmStore.DataStore.ConnectionLock.Lock()
	defer repo.fsmStore.DataStore.ConnectionLock.Unlock()
	tableName := ExecutionsUnCommittedTableName

	if committed {
		tableName = ExecutionsCommittedTableName
	}

	selectBuilder := sq.Select("count(*)").
		From(tableName).
		OrderBy(fmt.Sprintf("%s DESC", ExecutionsNextExecutionTime)).
		Limit(1).
		Where(fmt.Sprintf(
			"%s = ? AND %s = ? AND %s = ?",
			ExecutionsJobIdColumn,
			ExecutionsStateColumn,
			ExecutionsNodeIdColumn,
		),
			jobId,
			models.ExecutionLogFailedState,
			nodeId,
		).
		RunWith(repo.fsmStore.DataStore.Connection)

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Fatalln("failed to select last execution log", err)
	}
	var count uint64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if err != nil {
			repo.logger.Fatalln("failed to scan rows ", scanErr)
		}
	}
	if rows.Err() != nil {
		repo.logger.Fatalln("failed to select last execution log rows error", rows.Err())
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
		repo.logger.Fatalln("failed to select last execution log", err)
	}
	var count uint64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if err != nil {
			repo.logger.Fatalln("failed to scan rows ", scanErr)
		}
	}
	if rows.Err() != nil {
		repo.logger.Fatalln("failed to select last execution log rows error", rows.Err())
	}
	return count
}

func (repo *executionsRepo) GetUncommittedExecutionsLogForNode(nodeId int) []models.JobExecutionLog {
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
		repo.logger.Fatalln("failed to select last execution log")
	}
	results := []models.JobExecutionLog{}
	for rows.Next() {
		lastExecutionLog := models.JobExecutionLog{}
		var dateCreatedStr string
		var nextExecutionDTStr string
		var lastExecutionDTStr string
		err := rows.Scan(
			&lastExecutionLog.Id,
			&lastExecutionLog.UniqueId,
			&lastExecutionLog.State,
			&lastExecutionLog.NodeId,
			&lastExecutionDTStr,
			&nextExecutionDTStr,
			&lastExecutionLog.JobId,
			&dateCreatedStr,
			&lastExecutionLog.JobQueueVersion,
			&lastExecutionLog.ExecutionVersion,
		)
		t, errParse := dateparse.ParseLocal(dateCreatedStr)
		if errParse != nil {
			repo.logger.Fatalln("failed to parse date created string")
		}
		lastExecutionLog.DataCreated = t
		t, errParse = dateparse.ParseLocal(nextExecutionDTStr)
		if errParse != nil {
			repo.logger.Fatalln("failed to parse next execution date time string")
		}
		lastExecutionLog.NextExecutionDatetime = t
		t, errParse = dateparse.ParseLocal(lastExecutionDTStr)
		if errParse != nil {
			repo.logger.Fatalln("failed to last execution date time string")
		}
		lastExecutionLog.LastExecutionDatetime = t
		if err != nil {
			repo.logger.Fatalln("failed to scan rows")
		}
		results = append(results, lastExecutionLog)
	}
	if rows.Err() != nil {
		repo.logger.Fatalln("failed to select last execution log rows error")
	}

	return results
}
