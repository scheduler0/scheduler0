package process

import (
	"scheduler0/server/managers/execution"
	"scheduler0/server/transformers"
)

// PendingJob job
type PendingJob struct {
	Job *transformers.Job
	Execution *execution.Manager
}