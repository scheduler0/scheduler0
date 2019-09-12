package dto

import (
	"encoding/json"
	"time"
)

type JobState int

const (
	INACTIVE_JOB JobState = iota + 1
	ACTIVE_JOB
)

type Job struct {
	ID          int64     `json:"id"`
	Cron        string    `json:"cron"`
	ServiceName string    `json:"service_name"`
	Data        string    `json:"data"`
	State       JobState  `json:"state"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	NextTime    time.Time `json:"next_time"`
}

func (j *Job) ToJson() string {
	job, err := json.Marshal(j)

	if err != nil {
		panic(err)
	}

	return string(job)
}

func JobFromJson(body []byte) Job {
	job := Job{}
	err := json.Unmarshal(body, &job)

	if err != nil {
		panic(err)
	}

	return job
}
