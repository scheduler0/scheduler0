package job_execution

import (
	"context"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/hashicorp/go-hclog"
	"github.com/robfig/cron"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/models"
	"scheduler0/pkg/scheduler0time"
	"scheduler0/pkg/utils"
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

//go:generate mockery --name JobExecutionsRepo --output ../mocks
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

	batches := utils.Batch[models.Job](jobs, 9)
	var returningIds []uint64

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
		var params []interface{}
		var ids []uint64

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
			schedulerTime := scheduler0time.GetSchedulerTime()
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
		tx, err := repo.fsmStore.GetDataStore().GetOpenConnection().BeginTx(ctx, nil)
		if err != nil {
			repo.logger.Error("failed to create transaction for batch insertion", err)
			return
		}

		res, err := tx.Exec(query, params...)
		if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				repo.logger.Error("failed to rollback failed batch insertion execute ", "error", err)
				return
			} else {
				repo.logger.Error("failed to execute batch insertion", "error", err)
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
				repo.logger.Error("failed to rollback, failed batch insertion execute, failed to get last inserted id", "error", err)
				return
			} else {
				repo.logger.Error("failed to execute batch insertion, failed to get last inserted id", "error", err)
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

		rows, err := repo.fsmStore.GetDataStore().GetOpenConnection().Query(query, params...)
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
		rows.Close()
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
	defer rows.Close()
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
	defer rows.Close()
	if err != nil {
		repo.logger.Error("failed to count executions log", err)
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
		repo.logger.Error("failed to count execution logs error", rows.Err())
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
	defer rows.Close()
	if err != nil {
		repo.logger.Error("failed to count async tasks rows", err)
		return 0, 0
	}
	var minId uint64 = 0
	var maxId uint64 = 0
	for rows.Next() {
		scanErr := rows.Scan(&minId, &maxId)
		if err != nil {
			repo.logger.Error("failed to scan rows ", scanErr)
			return 0, 0
		}
	}
	if rows.Err() != nil {
		repo.logger.Error("failed to count async tasks rows error", rows.Err())
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
		).
			From(ExecutionsUnCommittedTableName).
			OrderBy(fmt.Sprintf("%s DESC", ExecutionsNextExecutionTime)).
			Where(fmt.Sprintf("%s = ? AND %s in (%s)", ExecutionsNodeIdColumn, ExecutionsIdColumn, paramPlaceholders), params...).
			Limit(constants.JobExecutionLogMaxBatchSize).
			RunWith(repo.fsmStore.GetDataStore().GetOpenConnection())

		rows, err := selectBuilder.Query()
		if err != nil {
			repo.logger.Error("failed to select last execution log", err)
			return nil
		}
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
		rows.Close()
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
			repo.logger.Error("failed to get next execution time", "error", err)
			continue
		}
		executionLogs = append(executionLogs, models.JobExecutionLog{
			JobId:                 job.ID,
			UniqueId:              job.ExecutionId,
			State:                 state,
			NodeId:                nodeId,
			LastExecutionDatetime: job.LastExecutionDate,
			NextExecutionDatetime: *executionTime,
			JobQueueVersion:       lastVersion,
			DataCreated:           now,
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
		query := fmt.Sprintf("INSERT INTO %s (%s, %s, %s, %s, %s, %s, %s , %s, %s) VALUES ",
			ExecutionsCommittedTableName,
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

		for i, executionLog := range batch {
			query += fmt.Sprint("(?, ?, ?, ?, ?, ?, ?, ?, ?)")
			params = append(params,
				executionLog.UniqueId,
				executionLog.State,
				executionLog.NodeId,
				executionLog.LastExecutionDatetime,
				executionLog.NextExecutionDatetime,
				executionLog.JobId,
				executionLog.JobQueueVersion,
				executionLog.DataCreated,
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
