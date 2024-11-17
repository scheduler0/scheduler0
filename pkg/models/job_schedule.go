package models

import "time"

type JobSchedule struct {
	Job           Job       `json:"job"`
	ExecutionTime time.Time `json:"executionTime"`
}
