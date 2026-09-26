package job_execution

import (
	"context"
	"fmt"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/utils"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"github.com/robfig/cron"
)

const (
	ExecutionsCommittedTableName   = "job_executions_committed"
	ExecutionsUnCommittedTableName = "job_executions_uncommitted"
)

const (
	ExecutionsIdColumn                = "id"
	ExecutionsAccountIdColumn         = "account_id"
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

type JobExecutionsRepo interface {
	BatchInsert(jobs []models.Job, nodeId uint64, state models.JobExecutionLogState, jobQueueVersion uint64, executionVersions map[uint64]uint64)
	CountLastFailedExecutionLogs(jobId uint64, nodeId uint64, executionVersion uint64) uint64
	CountExecutionLogs(committed bool) uint64
	GetUncommittedExecutionsLogForNode(nodeId uint64) []models.JobExecutionLog
	GetLastExecutionLogForJobIds(jobIds []uint64) map[uint64]models.JobExecutionLog
	LogJobExecutionStateInRaft(
		jobs []models.Job,
		state models.JobExecutionLogState,
		executionVersions map[uint64]uint64,
		lastVersion uint64,
		nodeId uint64,
	)
	RaftInsertExecutionLogs(executionLogs []models.JobExecutionLog, nodeId uint64)
	GetExecutionLogsFiltered(accountId uint64, startDate, endDate *time.Time, projectId *uint64, jobId *uint64, state *models.JobExecutionLogState, orderBy string, orderDirection string) ([]models.JobExecutionLog, error)
	// GetExecutionUsageByAccountIds returns, for each accountId, the number of executions
	// recorded in the committed executions table starting from the provided startDate
	// (typically from the most recent job queue's date_created) and counting up to the current time.
	// This is intended for quota reconciliation on the leader.
	GetExecutionUsageByAccountIds(accountIds []uint64, startDate time.Time) (map[uint64]uint64, error)
	// DeleteOldExecutionLogs deletes execution logs older than 30 days for accounts
	// without the increased retention feature and older than 90 days for accounts
	// with the feature.
	DeleteOldExecutionLogs() error
	// DeleteOldExecutionLogsForAccount deletes execution logs older than retentionDays for a specific account
	DeleteOldExecutionLogsForAccount(accountId uint64, retentionDays int) error
	// DeleteUncommittedExecutionLogsByIdRange deletes uncommitted execution logs
	// within the specified ID range for the given node
	DeleteUncommittedExecutionLogsByIdRange(minId, maxId int64, nodeId uint64) error
	// GetDateRangeAnalytics returns execution counts grouped by minute buckets for a date range
	// Accepts accountId, startDate, startTime (both in UTC)
	// Automatically calculates window as selected datetime ± 7 hours (14 hour total window)
	// All times are in UTC - timezone conversion should be done on the frontend
	GetDateRangeAnalytics(accountId uint64, startDate, startTime time.Time) (*models.DateRangeAnalyticsResponse, error)
	// GetExecutionTotals returns total counts of scheduled, success, and failed executions for an account
	GetExecutionTotals(accountId uint64) (*models.ExecutionTotalsResponse, error)
}

type executionsRepo struct {
	fsmStore              fsm.Scheduler0RaftStore
	logger                hclog.Logger
	scheduler0RaftActions fsm.Scheduler0RaftActions
}

func NewExecutionsRepo(logger hclog.Logger, scheduler0RaftActions fsm.Scheduler0RaftActions, store fsm.Scheduler0RaftStore) *executionsRepo {
	return &executionsRepo{
		fsmStore:              store,
		logger:                logger.Named("job-executions-repo"),
		scheduler0RaftActions: scheduler0RaftActions,
	}
}

func (repo *executionsRepo) BatchInsert(jobs []models.Job, nodeId uint64, state models.JobExecutionLogState, jobQueueVersion uint64, jobExecutionVersions map[uint64]uint64) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	if len(jobs) < 1 {
		return
	}

	batches := utils.Batch[models.Job](jobs, 10)
	var returningIds []uint64

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s, %s , %s, %s) VALUES ",
			ExecutionsUnCommittedTableName,
			ExecutionsAccountIdColumn,
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
		var params []interface{}
		var ids []uint64

		for i, job := range batch {
			executionVersion := 0

			if jobExecutionVersion, ok := jobExecutionVersions[uint64(job.ID)]; ok {
				executionVersion = int(jobExecutionVersion)
			}

			query += fmt.Sprint("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			executionTime := time.Time{}
			now := scheduler0time.GetSchedulerTime().GetTime(time.Now())
			if job.Spec != "" {
				schedule, parseErr := cron.Parse(job.Spec)
				if parseErr != nil {
					repo.logger.Error("BatchInsert: failed to parse job cron spec", "error", parseErr, "jobId", job.ID, "spec", job.Spec)
				}
				executionTime = schedule.Next(jobs[i].LastExecutionDate)
			} else {
				executionTime = job.StartDate
			}
			params = append(params,
				job.AccountId,
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
		tx, err := repo.fsmStore.GetDataStore().GetOpenConnection().BeginTx(ctx, nil)
		if err != nil {
			repo.logger.Error("BatchInsert: failed to create transaction for batch insertion", "error", err, "batchSize", len(batch), "nodeId", nodeId)
			return
		}

		res, err := tx.Exec(query, params...)
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				repo.logger.Error("BatchInsert: failed to rollback failed batch insertion execute", "error", err, "rollbackError", rollbackErr, "batchSize", len(batch), "nodeId", nodeId)
				return
			} else {
				repo.logger.Error("BatchInsert: failed to execute batch insertion", "error", err, "batchSize", len(batch), "nodeId", nodeId)
				return
			}
		}
		err = tx.Commit()
		if err != nil {
			repo.logger.Error("BatchInsert: failed to commit execute batch insertion", "error", err, "batchSize", len(batch), "nodeId", nodeId)
			return
		}

		lastInsertedId, err := res.LastInsertId()
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				repo.logger.Error("BatchInsert: failed to rollback, failed batch insertion execute, failed to get last inserted id", "error", err, "rollbackError", rollbackErr, "batchSize", len(batch), "nodeId", nodeId)
				return
			} else {
				repo.logger.Error("BatchInsert: failed to execute batch insertion, failed to get last inserted id", "error", err, "batchSize", len(batch), "nodeId", nodeId)
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
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	var results []models.JobExecutionLog

	if len(jobIds) < 1 {
		return results
	}

	batches := utils.Batch[uint64](jobIds, 1)

	for _, batch := range batches {
		paramsPlaceholder := "?"
		params := []interface{}{batch[0]}

		for _, jobId := range batch[1:] {
			paramsPlaceholder += ",?"
			params = append(params, jobId)
		}

		query := fmt.Sprintf(
			"select %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s from (select %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, row_number() over (partition by job_id order by execution_version desc, state desc) rowNum from (select %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s from job_executions_committed union all select %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s from job_executions_uncommitted order by job_queue_version desc) where %s in (%s)) t where t.rowNum = 1",

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
			ExecutionsAccountIdColumn,

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
			ExecutionsAccountIdColumn,

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
			ExecutionsAccountIdColumn,

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
			ExecutionsAccountIdColumn,

			ExecutionsJobIdColumn,

			paramsPlaceholder,
		)

		rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, params...)
		if err != nil {
			repo.logger.Error("getLastExecutionLogForJobIds: failed to select last execution log", "error", err, "batchSize", len(batch))
			return nil
		}
		defer rows.Close()
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
				&lastExecutionLog.DateCreated,
				&lastExecutionLog.JobQueueVersion,
				&lastExecutionLog.AccountId,
			)
			if scanErr != nil {
				repo.logger.Error("getLastExecutionLogForJobIds: failed to scan rows", "error", scanErr, "batchSize", len(batch))
				return nil
			}
			results = append(results, lastExecutionLog)
		}
		if rows.Err() != nil {
			repo.logger.Error("getLastExecutionLogForJobIds: failed to select last execution log rows error", "error", rows.Err(), "batchSize", len(batch))
			return nil
		}
	}

	return results
}

func (repo *executionsRepo) GetLastExecutionLogForJobIds(jobIds []uint64) map[uint64]models.JobExecutionLog {
	lastCommittedExecutionLogs := repo.getLastExecutionLogForJobIds(jobIds)

	executionLogsMap := make(map[uint64]models.JobExecutionLog, len(jobIds))

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
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	query := fmt.Sprintf("select count(*) from ("+
		"select * from job_executions_committed union all select * from job_executions_uncommitted"+
		") where %s = ? AND %s = ? AND %s = ? AND %s = ? order by %s desc limit 1",
		ExecutionsJobIdColumn,
		ExecutionsVersion,
		ExecutionsNodeIdColumn,
		ExecutionsStateColumn,
		ExecutionsVersion,
	)

	rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, jobId, executionVersion, nodeId, models.ExecutionLogFailedState)
	if err != nil {
		repo.logger.Error("CountLastFailedExecutionLogs: failed to select last execution log", "error", err, "jobId", jobId, "nodeId", nodeId, "executionVersion", executionVersion)
		return 0
	}
	defer rows.Close()
	var count uint64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if scanErr != nil {
			repo.logger.Error("CountLastFailedExecutionLogs: failed to scan rows", "error", scanErr, "jobId", jobId, "nodeId", nodeId, "executionVersion", executionVersion)
			return 0
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("CountLastFailedExecutionLogs: failed to select last execution log rows error", "error", rows.Err(), "jobId", jobId, "nodeId", nodeId, "executionVersion", executionVersion)
		return 0
	}
	return count
}

func (repo *executionsRepo) CountExecutionLogs(committed bool) uint64 {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	tableName := ExecutionsUnCommittedTableName

	if committed {
		tableName = ExecutionsCommittedTableName
	}

	selectBuilder := sq.Select("count(*)").
		From(tableName).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("CountExecutionLogs: failed to count executions log", "error", err, "committed", committed)
		return 0
	}
	defer rows.Close()
	var count uint64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&count)
		if scanErr != nil {
			repo.logger.Error("CountExecutionLogs: failed to scan rows", "error", scanErr, "committed", committed)
			return 0
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("CountExecutionLogs: failed to count execution logs error", "error", rows.Err(), "committed", committed)
		return 0
	}
	return count
}

func (repo *executionsRepo) getUncommittedExecutionsLogsMinMaxIds(committed bool) (uint64, uint64) {
	tableName := ExecutionsUnCommittedTableName

	if committed {
		tableName = ExecutionsCommittedTableName
	}

	selectBuilder := sq.Select("min(id)", "max(id)").
		From(tableName).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("getUncommittedExecutionsLogsMinMaxIds: failed to query min/max ids", "error", err, "committed", committed)
		return 0, 0
	}
	defer rows.Close()
	var minId uint64 = 0
	var maxId uint64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&minId, &maxId)
		if scanErr != nil {
			repo.logger.Error("getUncommittedExecutionsLogsMinMaxIds: failed to scan rows", "error", scanErr, "committed", committed)
			return 0, 0
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("getUncommittedExecutionsLogsMinMaxIds: failed to query min/max ids rows error", "error", rows.Err(), "committed", committed)
		return 0, 0
	}
	return minId, maxId
}

func (repo *executionsRepo) GetUncommittedExecutionsLogForNode(nodeId uint64) []models.JobExecutionLog {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	min, max := repo.getUncommittedExecutionsLogsMinMaxIds(false)
	ids := utils.ExpandIdsRange(min, max)
	batches := utils.Batch(ids, 10)
	var results []models.JobExecutionLog

	for _, batch := range batches {
		var params = []interface{}{nodeId, batch[0]}
		var paramPlaceholders = "?"

		for _, b := range batch[1:] {
			paramPlaceholders += ",?"
			params = append(params, b)
		}

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
			ExecutionsAccountIdColumn,
		).
			From(ExecutionsUnCommittedTableName).
			OrderBy(fmt.Sprintf("%s DESC", ExecutionsNextExecutionTime)).
			Where(fmt.Sprintf("%s = ? AND %s in (%s)", ExecutionsNodeIdColumn, ExecutionsIdColumn, paramPlaceholders), params...).
			Limit(constants.JobExecutionLogMaxBatchSize).
			RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

		rows, err := selectBuilder.Query()
		if err != nil {
			repo.logger.Error("GetUncommittedExecutionsLogForNode: failed to select last execution log", "error", err, "nodeId", nodeId, "batchSize", len(batch))
			return nil
		}
		defer rows.Close()
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
				&lastExecutionLog.DateCreated,
				&lastExecutionLog.JobQueueVersion,
				&lastExecutionLog.ExecutionVersion,
				&lastExecutionLog.AccountId,
			)
			if scanErr != nil {
				repo.logger.Error("GetUncommittedExecutionsLogForNode: failed to scan rows", "error", scanErr, "nodeId", nodeId)
				return nil
			}
			results = append(results, lastExecutionLog)
		}
		if rows.Err() != nil {
			repo.logger.Error("GetUncommittedExecutionsLogForNode: failed to select last execution log rows error", "error", rows.Err(), "nodeId", nodeId)
			return nil

		}
	}

	return results
}

func (repo *executionsRepo) LogJobExecutionStateInRaft(
	jobs []models.Job,
	state models.JobExecutionLogState,
	executionVersions map[uint64]uint64,
	lastVersion uint64,
	nodeId uint64,
) {
	executionLogs := make([]models.JobExecutionLog, 0, len(jobs))

	for _, job := range jobs {
		sched := scheduler0time.GetSchedulerTime()
		now := sched.GetTime(time.Now())
		executionTime, err := job.GetNextExecutionTime()
		if err != nil {
			repo.logger.Error("LogJobExecutionStateInRaft: failed to get next execution time", "error", err, "jobId", job.ID, "nodeId", nodeId)
			continue
		}
		executionLogs = append(executionLogs, models.JobExecutionLog{
			AccountId:             job.AccountId,
			JobId:                 job.ID,
			UniqueId:              job.ExecutionId,
			State:                 state,
			NodeId:                nodeId,
			LastExecutionDatetime: job.LastExecutionDate,
			NextExecutionDatetime: *executionTime,
			JobQueueVersion:       lastVersion,
			DateCreated:           now,
			ExecutionVersion:      executionVersions[job.ID],
		})
	}

	repo.RaftInsertExecutionLogs(executionLogs, nodeId)
}

func (repo *executionsRepo) RaftInsertExecutionLogs(executionLogs []models.JobExecutionLog, nodeId uint64) {
	if len(executionLogs) < 1 {
		return
	}

	batches := utils.Batch[models.JobExecutionLog](executionLogs, 9)

	for _, batch := range batches {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s , %s, %s, %s) VALUES ",
			ExecutionsCommittedTableName,
			ExecutionsUniqueIdColumn,
			ExecutionsAccountIdColumn,
			ExecutionsStateColumn,
			ExecutionsNodeIdColumn,
			ExecutionsLastExecutionTimeColumn,
			ExecutionsNextExecutionTime,
			ExecutionsJobIdColumn,
			ExecutionsJobQueueVersion,
			ExecutionsDateCreatedColumn,
			ExecutionsVersion,
		)
		var params []interface{}

		for i, executionLog := range batch {
			query += "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
			params = append(params,
				executionLog.UniqueId,
				executionLog.AccountId,
				executionLog.State,
				executionLog.NodeId,
				executionLog.LastExecutionDatetime,
				executionLog.NextExecutionDatetime,
				executionLog.JobId,
				executionLog.JobQueueVersion,
				executionLog.DateCreated,
				executionLog.ExecutionVersion,
			)
			if i < len(batch)-1 {
				query += ","
			}
		}

		query += ";"

		repo.scheduler0RaftActions.WriteCommandToRaftLog(
			repo.fsmStore.GetRaft(),
			constants.CommandTypeDbExecute,
			query,
			params,
			[]uint64{nodeId},
			constants.CommandActionCleanUncommittedExecutionLogs,
		)
	}
}

// GetExecutionLogsFiltered returns both committed and uncommitted logs filtered by accountId, date range, and optionally projectId, jobId, and state
func (repo *executionsRepo) GetExecutionLogsFiltered(accountId uint64, startDate, endDate *time.Time, projectId *uint64, jobId *uint64, state *models.JobExecutionLogState, orderBy string, orderDirection string) ([]models.JobExecutionLog, error) {
	repo.logger.Info("GetExecutionLogsFiltered entry",
		"accountId", accountId,
		"startDate", startDate,
		"endDate", endDate,
		"projectId", projectId,
		"jobId", jobId,
		"state", state,
		"orderBy", orderBy,
		"orderDirection", orderDirection)

	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	logs := []models.JobExecutionLog{}

	// Map sort field names to column names
	var sortColumn string
	switch orderBy {
	case "dateCreated":
		sortColumn = ExecutionsDateCreatedColumn
	case "lastExecutionDateTime":
		sortColumn = ExecutionsLastExecutionTimeColumn
	case "nextExecutionDateTime":
		sortColumn = ExecutionsNextExecutionTime
	default:
		// Default to dateCreated DESC if invalid or empty
		sortColumn = ExecutionsDateCreatedColumn
		orderDirection = "DESC"
	}

	// Validate order direction
	if orderDirection != "ASC" && orderDirection != "DESC" {
		orderDirection = "DESC"
	}

	selectBuilder := sq.Select(
		ExecutionsIdColumn,
		ExecutionsUniqueIdColumn,
		ExecutionsStateColumn,
		ExecutionsNodeIdColumn,
		ExecutionsLastExecutionTimeColumn,
		ExecutionsNextExecutionTime,
		ExecutionsJobIdColumn,
		ExecutionsJobQueueVersion,
		ExecutionsVersion,
		ExecutionsDateCreatedColumn,
		ExecutionsAccountIdColumn,
	).From(ExecutionsCommittedTableName).
		Where(fmt.Sprintf("%s = ?", ExecutionsAccountIdColumn), accountId).
		RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

	// Add date filters only if provided
	// Use datetime() function for proper date comparison and >= / <= for inclusive range
	if startDate != nil {
		selectBuilder = selectBuilder.Where(fmt.Sprintf("datetime(%s) >= datetime(?)", ExecutionsDateCreatedColumn), (*startDate).Format(time.RFC3339))
	}
	if endDate != nil {
		selectBuilder = selectBuilder.Where(fmt.Sprintf("datetime(%s) <= datetime(?)", ExecutionsDateCreatedColumn), (*endDate).Format(time.RFC3339))
	}

	if projectId != nil {
		selectBuilder = selectBuilder.Where("job_id IN (SELECT id FROM jobs WHERE project_id = ?)", *projectId)
	}
	if jobId != nil {
		selectBuilder = selectBuilder.Where(fmt.Sprintf("%s = ?", ExecutionsJobIdColumn), *jobId)
	}
	if state != nil {
		selectBuilder = selectBuilder.Where(fmt.Sprintf("%s = ?", ExecutionsStateColumn), *state)
	}

	// Add ORDER BY clause
	selectBuilder = selectBuilder.OrderBy(fmt.Sprintf("%s %s", sortColumn, orderDirection))

	rows, err := selectBuilder.Query()
	if err != nil {
		repo.logger.Error("GetExecutionLogsFiltered: failed to query job execution logs", "error", err, "accountId", accountId, "table", constants.ExecutionsCommittedTableName, "startDate", startDate, "endDate", endDate)
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var log models.JobExecutionLog
		scanErr := rows.Scan(
			&log.Id,
			&log.UniqueId,
			&log.State,
			&log.NodeId,
			&log.LastExecutionDatetime,
			&log.NextExecutionDatetime,
			&log.JobId,
			&log.JobQueueVersion,
			&log.ExecutionVersion,
			&log.DateCreated,
			&log.AccountId,
		)
		if scanErr != nil {
			repo.logger.Error("GetExecutionLogsFiltered: failed to scan job execution log row", "error", scanErr, "accountId", accountId, "table", constants.ExecutionsCommittedTableName)
			return nil, scanErr
		}
		logs = append(logs, log)
	}
	if rows.Err() != nil {
		repo.logger.Error("GetExecutionLogsFiltered: row iteration error", "error", rows.Err(), "accountId", accountId, "table", constants.ExecutionsCommittedTableName)
		return nil, rows.Err()
	}

	return logs, nil
}

// GetExecutionUsageByAccountIds returns, for each accountId, the number of executions
// recorded in the committed executions table within the provided date range.
// This is intended for quota reconciliation on the leader and only looks at
// the committed executions table to avoid double-counting uncommitted logs.
func (repo *executionsRepo) GetExecutionUsageByAccountIds(accountIds []uint64, startDate time.Time) (map[uint64]uint64, error) {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	usage := make(map[uint64]uint64)

	if len(accountIds) == 0 {
		return usage, nil
	}

	// Build placeholders and params for IN clause
	params := make([]interface{}, 0, len(accountIds)+1)
	var paramPlaceholders string

	if len(accountIds) > 0 {
		params = append(params, accountIds[0])
		paramPlaceholders = "?"
		for _, accountId := range accountIds[1:] {
			paramPlaceholders += ",?"
			params = append(params, accountId)
		}
	}

	// Append start date param (end date is implicitly "now" - we count from startDate onwards)
	params = append(params, startDate)

	query := fmt.Sprintf(
		"SELECT %s, COUNT(*) FROM %s WHERE %s IN (%s) AND %s >= ? GROUP BY %s",
		ExecutionsAccountIdColumn,
		ExecutionsCommittedTableName,
		ExecutionsAccountIdColumn,
		paramPlaceholders,
		ExecutionsDateCreatedColumn,
		ExecutionsAccountIdColumn,
	)

	rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, params...)
	if err != nil {
		repo.logger.Error("GetExecutionUsageByAccountIds: failed to query execution usage by account ids", "error", err, "accountIdsCount", len(accountIds), "startDate", startDate)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var accountId uint64
		var count uint64
		if scanErr := rows.Scan(&accountId, &count); scanErr != nil {
			repo.logger.Error("GetExecutionUsageByAccountIds: failed to scan row", "error", scanErr)
			return nil, scanErr
		}
		usage[accountId] = count
	}

	if rows.Err() != nil {
		repo.logger.Error("GetExecutionUsageByAccountIds: row iteration error", "error", rows.Err())
		return nil, rows.Err()
	}

	return usage, nil
}

// DeleteOldExecutionLogs cleans up committed and uncommitted execution logs based on retention policy.
// Accounts WITH IncreasedExecutionLogs90DaysRetentionFeature keep 90 days; others keep 30 days.
func (repo *executionsRepo) DeleteOldExecutionLogs() error {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	sched := scheduler0time.GetSchedulerTime()
	now := sched.GetTime(time.Now())
	cutoff30 := now.Add(-30 * 24 * time.Hour)
	cutoff90 := now.Add(-90 * 24 * time.Hour)

	featureName := constants.IncreasedExecutionLogs90DaysRetentionFeature

	deleteOlderThanForNonFeature := func(table string, cutoff time.Time) error {
		query := fmt.Sprintf(
			"DELETE FROM %s WHERE %s < ? AND %s NOT IN (SELECT account_id FROM account_features af JOIN features f ON f.id = af.feature_id WHERE f.name = ?)",
			table, ExecutionsDateCreatedColumn, ExecutionsAccountIdColumn,
		)
		_, err := repo.fsmStore.GetDataStore().GetOpenConnection().Exec(query, cutoff, featureName)
		return err
	}

	deleteOlderThanForFeature := func(table string, cutoff time.Time) error {
		query := fmt.Sprintf(
			"DELETE FROM %s WHERE %s < ? AND %s IN (SELECT account_id FROM account_features af JOIN features f ON f.id = af.feature_id WHERE f.name = ?)",
			table, ExecutionsDateCreatedColumn, ExecutionsAccountIdColumn,
		)
		_, err := repo.fsmStore.GetDataStore().GetOpenConnection().Exec(query, cutoff, featureName)
		return err
	}

	for _, table := range []string{ExecutionsCommittedTableName, ExecutionsUnCommittedTableName} {
		if err := deleteOlderThanForNonFeature(table, cutoff30); err != nil {
			repo.logger.Error("DeleteOldExecutionLogs: failed to delete old execution logs (30d)", "table", table, "error", err)
			return err
		}
		if err := deleteOlderThanForFeature(table, cutoff90); err != nil {
			repo.logger.Error("DeleteOldExecutionLogs: failed to delete old execution logs (90d)", "table", table, "error", err)
			return err
		}
	}

	return nil
}

// DeleteOldExecutionLogsForAccount deletes execution logs older than retentionDays for a specific account
func (repo *executionsRepo) DeleteOldExecutionLogsForAccount(accountId uint64, retentionDays int) error {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	sched := scheduler0time.GetSchedulerTime()
	now := sched.GetTime(time.Now())
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour)

	repo.logger.Debug("DeleteOldExecutionLogsForAccount: cleaning up execution logs", "accountId", accountId, "retentionDays", retentionDays, "cutoff", cutoff)

	deleteOlderThanForAccount := func(table string) error {
		query := fmt.Sprintf(
			"DELETE FROM %s WHERE %s < ? AND %s = ?",
			table, ExecutionsDateCreatedColumn, ExecutionsAccountIdColumn,
		)
		_, err := repo.fsmStore.GetDataStore().GetOpenConnection().Exec(query, cutoff, accountId)
		return err
	}

	for _, table := range []string{ExecutionsCommittedTableName, ExecutionsUnCommittedTableName} {
		if err := deleteOlderThanForAccount(table); err != nil {
			repo.logger.Error("DeleteOldExecutionLogsForAccount: failed to delete old execution logs", "table", table, "accountId", accountId, "retentionDays", retentionDays, "error", err)
			return err
		}
	}

	repo.logger.Debug("DeleteOldExecutionLogsForAccount: successfully cleaned up execution logs", "accountId", accountId, "retentionDays", retentionDays)
	return nil
}

// DeleteUncommittedExecutionLogsByIdRange deletes uncommitted execution logs within the specified ID range for the given node
func (repo *executionsRepo) DeleteUncommittedExecutionLogsByIdRange(minId, maxId int64, nodeId uint64) error {
	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	query := fmt.Sprintf(
		"DELETE FROM %s WHERE %s >= ? AND %s <= ? AND %s = ?",
		ExecutionsUnCommittedTableName,
		ExecutionsIdColumn,
		ExecutionsIdColumn,
		ExecutionsNodeIdColumn,
	)

	_, err := repo.fsmStore.GetDataStore().GetOpenConnection().Exec(query, minId, maxId, nodeId)
	if err != nil {
		repo.logger.Error("DeleteUncommittedExecutionLogsByIdRange: failed to delete uncommitted execution logs by ID range",
			"minId", minId,
			"maxId", maxId,
			"nodeId", nodeId,
			"error", err)
		return err
	}

	repo.logger.Debug("DeleteUncommittedExecutionLogsByIdRange: successfully deleted uncommitted execution logs by ID range",
		"minId", minId,
		"maxId", maxId,
		"nodeId", nodeId)

	return nil
}

// GetDateRangeAnalytics returns execution counts grouped by hour buckets for a date range
// Accepts accountId, startDate, startTime (both in UTC)
// Automatically calculates window as selected datetime ± 6 hours (12 hour total window)
// All times are in UTC - timezone conversion should be done on the frontend
func (repo *executionsRepo) GetDateRangeAnalytics(
	accountId uint64,
	startDate, startTime time.Time,
) (*models.DateRangeAnalyticsResponse, error) {
	repo.logger.Info("GetDateRangeAnalytics: entry",
		"accountId", accountId,
		"startDate", startDate.Format("2006-01-02"),
		"startTime", startTime.Format("15:04:05"))

	// Combine startDate and startTime into a single time.Time (both should already be in UTC)
	// This represents the selected/center datetime
	selectedDateTime := time.Date(
		startDate.Year(), startDate.Month(), startDate.Day(),
		startTime.Hour(), startTime.Minute(), startTime.Second(), 0,
		time.UTC,
	)

	// Calculate window: 6 hours before and 6 hours after the selected datetime (12 hour total)
	startDateTime := selectedDateTime.Add(-6 * time.Hour)
	endDateTime := selectedDateTime.Add(6 * time.Hour)

	repo.logger.Debug("GetDateRangeAnalytics: calculated date range",
		"selectedDateTime", selectedDateTime.Format(time.RFC3339),
		"startDateTime", startDateTime.Format(time.RFC3339),
		"endDateTime", endDateTime.Format(time.RFC3339))

	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	// Create a map to store counts by hour bucket (key: "date:time")
	bucketMap := make(map[string]*models.DateRangeAnalyticsPoint)

	// Generate all hour buckets in the 12 hour window (6 hours before and after)
	currentHour := startDateTime.Truncate(time.Hour)
	for currentHour.Before(endDateTime) || currentHour.Equal(endDateTime) {
		dateStr := currentHour.Format("2006-01-02")
		timeStr := currentHour.Format("15:00:00")
		key := fmt.Sprintf("%s:%s", dateStr, timeStr)

		bucketMap[key] = &models.DateRangeAnalyticsPoint{
			Date:      dateStr,
			Time:      timeStr,
			Scheduled: 0,
			Success:   0,
			Failed:    0,
		}

		currentHour = currentHour.Add(time.Hour)
	}

	// Fetch execution logs from committed table only
	query := fmt.Sprintf(
		"SELECT %s, %s FROM %s WHERE %s = ? AND datetime(%s) >= datetime(?) AND datetime(%s) <= datetime(?)",
		ExecutionsDateCreatedColumn,
		ExecutionsStateColumn,
		ExecutionsCommittedTableName,
		ExecutionsAccountIdColumn,
		ExecutionsDateCreatedColumn,
		ExecutionsDateCreatedColumn,
	)

	startDateStr := startDateTime.UTC().Format("2006-01-02 15:04:05")
	endDateStr := endDateTime.UTC().Format("2006-01-02 15:04:05")

	repo.logger.Debug("GetDateRangeAnalytics: fetching committed logs",
		"startDateStr", startDateStr,
		"endDateStr", endDateStr)

	rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(
		query,
		accountId,
		startDateStr,
		endDateStr,
	)
	if err != nil {
		repo.logger.Error("GetDateRangeAnalytics: failed to query committed logs", "error", err)
		return nil, err
	}
	defer rows.Close()

	logCount := 0
	for rows.Next() {
		var dateCreated time.Time
		var state models.JobExecutionLogState

		if err := rows.Scan(&dateCreated, &state); err != nil {
			repo.logger.Error("GetDateRangeAnalytics: failed to scan log row", "error", err)
			return nil, err
		}

		// Round to hour bucket
		hourBucket := dateCreated.Truncate(time.Hour)
		dateStr := hourBucket.Format("2006-01-02")
		timeStr := hourBucket.Format("15:00:00")
		key := fmt.Sprintf("%s:%s", dateStr, timeStr)

		// Get or create bucket
		bucket, exists := bucketMap[key]
		if !exists {
			// Create bucket if it doesn't exist (shouldn't happen, but safety check)
			bucket = &models.DateRangeAnalyticsPoint{
				Date:      dateStr,
				Time:      timeStr,
				Scheduled: 0,
				Success:   0,
				Failed:    0,
			}
			bucketMap[key] = bucket
		}

		// Increment appropriate counter based on state
		switch state {
		case models.ExecutionLogScheduleState:
			bucket.Scheduled++
		case models.ExecutionLogSuccessState:
			bucket.Success++
		case models.ExecutionLogFailedState:
			bucket.Failed++
		}

		logCount++
	}

	if rows.Err() != nil {
		repo.logger.Error("GetDateRangeAnalytics: rows iteration error", "error", rows.Err())
		return nil, rows.Err()
	}

	// Convert map to sorted slice, filtering out points with all zeros
	var points []models.DateRangeAnalyticsPoint
	currentHour = startDateTime.Truncate(time.Hour)
	for currentHour.Before(endDateTime) || currentHour.Equal(endDateTime) {
		dateStr := currentHour.Format("2006-01-02")
		timeStr := currentHour.Format("15:00:00")
		key := fmt.Sprintf("%s:%s", dateStr, timeStr)

		if bucket, exists := bucketMap[key]; exists {
			// Only add points that have at least one non-zero value
			// if bucket.Scheduled > 0 || bucket.Success > 0 || bucket.Failed > 0 {
			points = append(points, *bucket)
			// }
		}
		// Skip empty buckets (all zeros)

		currentHour = currentHour.Add(time.Hour)
	}

	// Format response (all times in UTC)
	// Note: StartDate/StartTime represent the actual window start (selected - 6 hours)
	// EndDate/EndTime represent the actual window end (selected + 6 hours)
	response := &models.DateRangeAnalyticsResponse{
		AccountID: accountId,
		Timezone:  "UTC", // Always UTC - frontend handles timezone conversion
		StartDate: startDateTime.Format("2006-01-02"),
		StartTime: startDateTime.Format("15:00:00"),
		EndDate:   endDateTime.Format("2006-01-02"),
		EndTime:   endDateTime.Format("15:00:00"),
		Points:    points,
	}

	repo.logger.Info("GetDateRangeAnalytics: completed",
		"accountId", accountId,
		"pointsCount", len(points),
		"logsProcessed", logCount)

	return response, nil
}

// GetExecutionTotals returns total counts of scheduled, success, and failed executions for an account
func (repo *executionsRepo) GetExecutionTotals(accountId uint64) (*models.ExecutionTotalsResponse, error) {
	repo.logger.Info("GetExecutionTotals: entry", "accountId", accountId)

	repo.fsmStore.GetDataStore().ConnectionLock()
	defer repo.fsmStore.GetDataStore().ConnectionUnlock()

	// Query to get counts grouped by state
	query := fmt.Sprintf(
		"SELECT %s, COUNT(*) as count FROM %s WHERE %s = ? GROUP BY %s",
		ExecutionsStateColumn,
		ExecutionsCommittedTableName,
		ExecutionsAccountIdColumn,
		ExecutionsStateColumn,
	)

	rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, accountId)
	if err != nil {
		repo.logger.Error("GetExecutionTotals: failed to query execution totals", "error", err, "accountId", accountId)
		return nil, err
	}
	defer rows.Close()

	// Initialize counts
	var scheduled, success, failed uint64

	// Process rows
	for rows.Next() {
		var state models.JobExecutionLogState
		var count uint64

		if err := rows.Scan(&state, &count); err != nil {
			repo.logger.Error("GetExecutionTotals: failed to scan row", "error", err, "accountId", accountId)
			return nil, err
		}

		// Map state to count
		switch state {
		case models.ExecutionLogScheduleState:
			scheduled = count
		case models.ExecutionLogSuccessState:
			success = count
		case models.ExecutionLogFailedState:
			failed = count
		}
	}

	if rows.Err() != nil {
		repo.logger.Error("GetExecutionTotals: rows iteration error", "error", rows.Err(), "accountId", accountId)
		return nil, rows.Err()
	}

	response := &models.ExecutionTotalsResponse{
		AccountID: accountId,
		Scheduled: scheduled,
		Success:   success,
		Failed:    failed,
	}

	repo.logger.Info("GetExecutionTotals: completed",
		"accountId", accountId,
		"scheduled", scheduled,
		"success", success,
		"failed", failed)

	return response, nil
}
