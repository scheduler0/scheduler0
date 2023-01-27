package models

import "time"

type JobSchedule struct {
	Job           JobModel
	ExecutionTime time.Time
}
