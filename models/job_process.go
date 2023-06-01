package models

import (
	"github.com/robfig/cron"
)

type JobProcess struct {
	Job  *Job       `json:"job,omitempty"`
	Cron *cron.Cron `json:"cron,omitempty"`
}
