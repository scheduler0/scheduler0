package models

import (
	"github.com/robfig/cron"
)

type JobProcess struct {
	Job  *JobModel  `json:"job,omitempty"`
	Cron *cron.Cron `json:"cron,omitempty"`
}
