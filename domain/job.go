package domain

import (
	"cron/dto"
	"time"
)

type JobDomain struct {
	ID               int64
	TotalExecs       int64
	Cron             string
	ServiceName      string
	SecsBetweenExecs int64
	Data             string
	State            dto.JobState
	StartDate        time.Time
	EndDate          time.Time
	NextTime         time.Time
}
