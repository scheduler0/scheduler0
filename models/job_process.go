package models

import (
	"github.com/robfig/cron"
)

type JobProcess struct {
	Job  *JobModel
	Cron *cron.Cron
}
