package models

import "time"

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

	ID             string    `json:"id,omitempty" sql:",pk:notnull"`
	ProjectID      string    `json:"project_id" sql:",notnull"`
	Description    string    `json:"description" sql:",notnull"`
	CronSpec       string    `json:"cron_spec,omitempty" sql:",notnull"`
	TotalExecs     int64     `json:"total_execs,omitempty" sql:",notnull"`
	Data           string    `json:"data,omitempty"`
	CallbackUrl    string    `json:"callback_url" sql:",notnull"`
	LastStatusCode int       `json:"last_status_code"`
	Timezone       string    `json:"timezone"`
	State          State     `json:"state,omitempty" sql:",notnull"`
	StartDate      time.Time `json:"start_date,omitempty" sql:",notnull"`
	EndDate        time.Time `json:"end_date,omitempty"`
	NextTime       time.Time `json:"next_time,omitempty" sql:",notnull"`
	DateCreated    time.Time `json:"date_created" sql:",notnull,default:now()"`

	Project		ProjectModel `sql:",fk:project_id"`
}
