package models

import (
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

type JobModel struct {
	TableName struct{} `sql:"jobs"`

	ID             string    `json:"id,omitempty" pg:",notnull"`
	ProjectId      string    `json:"project_id" pg:",notnull"`
	Description    string    `json:"description" pg:",notnull"`
	CronSpec       string    `json:"cron_spec,omitempty" pg:",notnull"`
	TotalExecs     int64     `json:"total_execs,omitempty" pg:",notnull"`
	Data           string    `json:"transformers,omitempty"`
	CallbackUrl    string    `json:"callback_url" pg:",notnull"`
	LastStatusCode int       `json:"last_status_code"`
	Timezone       string    `json:"timezone"`
	State          State     `json:"state,omitempty" pg:",notnull"`
	StartDate      time.Time `json:"start_date,omitempty" pg:",notnull"`
	EndDate        time.Time `json:"end_date,omitempty"`
	NextTime       time.Time `json:"next_time,omitempty" pg:",notnull"`
	DateCreated    time.Time `json:"date_created" pg:",notnull"`
}