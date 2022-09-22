package process

import (
	"fmt"
	"github.com/robfig/cron"
	"scheduler0/server/models"
	"scheduler0/utils"
	"time"
)

// RecoveredJob job
type RecoveredJob struct {
	Job       *models.JobModel
	Execution *models.ExecutionModel
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

	processor.ExecutePendingJobs([]models.JobModel{*recoveredJob.Job})
}
