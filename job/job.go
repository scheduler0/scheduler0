package job

import (
	"encoding/json"
	"time"
)

/*
	State for a job could be:

	- InActive: 1
	- Active:   2
	- Stale:   -1

*/
type State int

const (
	InActiveJob State = iota + 1
	ActiveJob
	StaleJob
)

// Inbound and outbound data transfer object for job
type Dto struct {
	ID          string    `json:"id"`
	CronSpec    string    `json:"cron_spec"`
	ServiceName string    `json:"service_name"`
	Data        string    `json:"data"`
	NextTime    time.Time `json:"next_time"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
}

// Job domain internal representation of job
type Job struct {
	ID               string    `json:"id,omitempty"`
	CronSpec         string    `json:"cron_spec,omitempty"`
	ServiceName      string    `json:"id,omitempty"`
	TotalExecs       int64     `json:"total_execs,omitempty"`
	SecsBetweenExecs float64   `json:"secs_between_execs,omitempty"`
	Data             string    `json:"data,omitempty"`
	State            State     `json:"state,omitempty"`
	StartDate        time.Time `json:"total_execs,omitempty"`
	EndDate          time.Time `json:"end_date,omitempty"`
	NextTime         time.Time `json:"next_time,omitempty"`
}

func (j *Dto) ToDomain() Job {
	jd := Job{
		ID:          j.ID,
		CronSpec:    j.CronSpec,
		ServiceName: j.ServiceName,
		StartDate:   j.StartDate,
		EndDate:     j.EndDate,
		Data:        j.Data,
	}

	return jd
}

func (j *Dto) ToJson() string {
	job, err := json.Marshal(j)

	if err != nil {
		panic(err)
	}

	return string(job)
}

func FromJson(body []byte) Dto {
	job := Dto{}
	err := json.Unmarshal(body, &job)

	if err != nil {
		panic(err)
	}

	return job
}

func (jd *Job) ToDto() Dto {
	j := Dto{
		ID:          jd.ID,
		CronSpec:    jd.CronSpec,
		NextTime:    jd.NextTime,
		StartDate:   jd.StartDate,
		EndDate:     jd.EndDate,
		Data:        jd.Data,
		ServiceName: jd.ServiceName,
	}

	return j
}
