package models

import "time"

type JobExecutionLogState uint64

const (
	ExecutionLogScheduleState JobExecutionLogState = 0
	ExecutionLogSuccessState  JobExecutionLogState = 1
	ExecutionLogFailedState   JobExecutionLogState = 2
)

type JobExecutionLog struct {
	Id                    uint64               `json:"id" fake:"{number:1,100}"`
	UniqueId              string               `json:"uniqueId" fake:"{regex:[abcdef]{5}}"`
	State                 JobExecutionLogState `json:"state" fake:"{number:1,3}"`
	NodeId                uint64               `json:"nodeId" fake:"{number:1,100}"`
	LastExecutionDatetime time.Time            `json:"lastExecutionDatetime"`
	NextExecutionDatetime time.Time            `json:"nextExecutionDatetime"`
	JobId                 uint64               `json:"jobId" fake:"{number:1,100}"`
	JobQueueVersion       uint64               `json:"job_queue_version" fake:"{number:1,100}"`
	ExecutionVersion      uint64               `json:"execution_version" fake:"{number:1,100}"`
	DataCreated           time.Time            `json:"dataCreated"`
}

type MemJobExecution struct {
	ExecutionVersion      uint64
	FailCount             uint64
	LastState             JobExecutionLogState
	LastExecutionDatetime time.Time `json:"lastExecutionDatetime"`
	NextExecutionDatetime time.Time `json:"nextExecutionDatetime"`
}
