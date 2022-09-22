package process

import (
	"fmt"
	"github.com/robfig/cron"
	models2 "scheduler0/models"
	"scheduler0/utils"
	"time"
)

// RecoveredJob job
type RecoveredJob struct {
	Job       *models2.JobModel
	Execution *models2.ExecutionModel
}

func (recoveredJob *RecoveredJob) Run(processor *JobProcessor) {
	schedule, parseErr := cron.Parse(recoveredJob.Job.Spec)
	if parseErr != nil {
		utils.Error(fmt.Sprintf("Failed to parse spec %v", parseErr.Error()))
		return
	}
	now := time.Now().UTC()
	executionTime := schedule.Next(recoveredJob.Execution.TimeAdded).UTC()

	utils.Info(fmt.Sprintf("Recovered Job--ExecutionAdded %s jobID = %v should execute on %v which is %s from now",
		recoveredJob.Execution.TimeAdded,
		recoveredJob.Job.ID,
		executionTime.String(),
		executionTime.Sub(now),
	))

	time.Sleep(executionTime.Sub(now))

	processor.ExecutePendingJobs([]models2.JobModel{*recoveredJob.Job})
}
