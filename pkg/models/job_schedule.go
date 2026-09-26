package models

import "time"

type JobScheduleKey struct {
	JobId         uint64    `json:"jobId"`
	ExecutionTime time.Time `json:"executionTime"`
}

type JobSchedule struct {
	Job          Job             `json:"job"`
	MemExecution MemJobExecution `json:"memExecution"`
}
