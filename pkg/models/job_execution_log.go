package models

import "time"

type JobExecutionLogState uint64

const (
	ExecutionLogScheduleState JobExecutionLogState = 0
	ExecutionLogSuccessState  JobExecutionLogState = 1
	ExecutionLogFailedState   JobExecutionLogState = 2
)

// String labels used as HTTP query-param values and JSON filter values for execution state.
const (
	ExecutionStateScheduled = "scheduled"
	ExecutionStateSuccess   = "success"
	ExecutionStateFailed    = "failed"
)

type JobExecutionLog struct {
	Id                    uint64               `json:"id" fake:"{number:1,100}"`
	UniqueId              string               `json:"uniqueId" fake:"{regex:[abcdef]{5}}"`
	State                 JobExecutionLogState `json:"state" fake:"{number:1,3}"`
	NodeId                uint64               `json:"nodeId" fake:"{number:1,100}"`
	LastExecutionDatetime time.Time            `json:"lastExecutionDatetime" fake:"{date}"`
	NextExecutionDatetime time.Time            `json:"nextExecutionDatetime" fake:"{date}"`
	JobId                 uint64               `json:"jobId" fake:"{number:1,100}"`
	JobQueueVersion       uint64               `json:"jobQueueVersion" fake:"{number:1,100}"`
	ExecutionVersion      uint64               `json:"executionVersion" fake:"{number:1,100}"`
	DateCreated           time.Time            `json:"dateCreated" fake:"{date}"`
	AccountId             uint64               `json:"accountId" fake:"{number:1,100}"`
	DateModified          *time.Time           `json:"dateModified" fake:"{date}"`
}

type MemJobExecution struct {
	ExecutionVersion      uint64
	FailCount             uint64
	LastState             JobExecutionLogState
	LastExecutionDatetime time.Time `json:"lastExecutionDatetime"`
	NextExecutionDatetime time.Time `json:"nextExecutionDatetime"`
}

// PaginatedJobExecutionLog paginated container of execution logs
type PaginatedJobExecutionLog struct {
	Total  uint64            `json:"total"`
	Offset uint64            `json:"offset"`
	Limit  uint64            `json:"limit"`
	Data   []JobExecutionLog `json:"executions"`
}
