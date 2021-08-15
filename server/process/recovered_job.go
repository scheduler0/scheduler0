package process

import (
	"fmt"
	"github.com/robfig/cron"
	"scheduler0/server/transformers"
	"scheduler0/utils"
	"time"
)

// RecoveredJob job
type RecoveredJob struct {
	Job       *transformers.Job
	Execution *transformers.Execution
}

func (recoveredJob *RecoveredJob) Run(processor *JobProcessor) {
	schedule, parseErr := cron.Parse(recoveredJob.Job.Spec)
	if parseErr != nil {
		utils.Error(fmt.Sprintf("Failed to parse spec %v", parseErr.Error()))
		return
	}
	now := time.Now().UTC()
	executionTime := schedule.Next(recoveredJob.Execution.TimeAdded).UTC()

	utils.Info(fmt.Sprintf("Recovered Job--ExecutionAdded %s jobID = %s should execute on %s which is %s from now",
		recoveredJob.Execution.TimeAdded,
		recoveredJob.Job.UUID,
		executionTime.String(),
		executionTime.Sub(now),
	))

	time.Sleep(executionTime.Sub(now))

	executionManger, err := recoveredJob.Execution.ToManager()
	if err != nil {
		utils.Error(fmt.Sprintf("Failed to create execution manager %s", err.Error()))
	}

	processor.ExecuteHTTPJobs([]PendingJob{{
		Job:       recoveredJob.Job,
		Execution: &executionManger,
	}})
}
