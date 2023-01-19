package models

import "time"

type JobExecutionLogState uint64

const (
	ExecutionLogScheduleState JobExecutionLogState = 0
	ExecutionLogSuccessState                       = 1
	ExecutionLogFailedState                        = 2
)

type JobExecutionLog struct {
	Id                    uint64    `json:"id"`
	UniqueId              string    `json:"uniqueId"`
	State                 uint64    `json:"state"`
	NodeId                uint64    `json:"nodeId"`
	LastExecutionDatetime time.Time `json:"lastExecutionDatetime"`
	NextExecutionDatetime time.Time `json:"nextExecutionDatetime"`
	JobId                 uint64    `json:"jobId"`
	JobQueueVersion       uint64    `json:"job_queue_version"`
	ExecutionVersion      uint64    `json:"execution_version"`
	DataCreated           time.Time `json:"dataCreated"`
}

type CommitJobStateLog struct {
	Address string            `json:"address"`
	Logs    []JobExecutionLog `json:"logs"`
}

type MemJobExecution struct {
	ExecutionVersion      uint64
	FailCount             uint64
	LastState             JobExecutionLogState
	LastExecutionDatetime time.Time `json:"lastExecutionDatetime"`
	NextExecutionDatetime time.Time `json:"nextExecutionDatetime"`
}
