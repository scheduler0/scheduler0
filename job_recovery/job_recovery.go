package job_recovery

import (
	models "scheduler0/models"
)

type JobRecovery struct {
	RecoveredJobs []RecoveredJob
}

type RecoveredJob struct {
	Job       *models.JobModel
	Execution *models.ExecutionModel
}

//func (recoveredJob *RecoveredJob) Run() {
//	schedule, parseErr := cron.Parse(recoveredJob.Job.Spec)
//	if parseErr != nil {
//		utils.Error(fmt.Sprintf("Failed to parse spec %v", parseErr.Error()))
//		return
//	}
//	now := time.Now().UTC()
//	executionTime := schedule.Next(recoveredJob.Execution.TimeAdded).UTC()
//
//	utils.Info(fmt.Sprintf("Recovered Job--ExecutionAdded %s jobID = %v should execute on %v which is %s from now",
//		recoveredJob.Execution.TimeAdded,
//		recoveredJob.Job.ID,
//		executionTime.String(),
//		executionTime.Sub(now),
//	))
//
//	time.Sleep(executionTime.Sub(now))
//
//	onSuccess := func(pendingJobs []*process.JobProcess) {
//		for _, pendingJob := range pendingJobs {
//			//if jobProcessor.IsRecovered(pendingJob.ID) {
//			jobProcessor.RemoveJobRecovery(pendingJob.ID)
//			//	jobProcessor.AddJobs([]models.JobModel{*pendingJob}, nil)
//			//}
//
//			utils.Info(fmt.Sprintf("Executed job %v", pendingJob.Job.ID))
//		}
//	}
//
//	onFail := func(pj []*process.JobProcess, err error) {
//		utils.Error("HTTP REQUEST ERROR:: ", err.Error())
//	}
//
//	executorService := executor.NewService([]*process.JobProcess{
//		{
//			Job: recoveredJob.Job,
//		},
//	}, onSuccess, onFail)
//
//	executorService.ExecuteHTTP()
//}

// RecoverJobExecutions find jobs that could've not been executed due to timeout
//func (jobProcessor *JobRecovery) RecoverJobExecutions(jobTransformers []models.JobModel) {
//	manager := execution.Manager{}
//	for _, jobTransformer := range jobTransformers {
//		if jobProcessor.IsRecovered(jobTransformer.UUID) {
//			continue
//		}
//
//		count, err, executionManagers := manager.FindJobExecutionPlaceholderByUUID(jobProcessor.DBConnection, jobTransformer.UUID)
//		if err != nil {
//			utils.Error(fmt.Sprintf("Error occurred while fetching execution mangers for jobs to be recovered %s", err.Message))
//			continue
//		}
//
//		if count < 1 {
//			continue
//		}
//
//		executionManager := executionManagers[0]
//		schedule, parseErr := cron.Parse(jobTransformer.Spec)
//
//		if parseErr != nil {
//			utils.Error(fmt.Sprintf("Failed to create schedule%s", parseErr.Error()))
//			continue
//		}
//		now := time.Now().UTC()
//		executionTime := schedule.Next(executionManager.TimeAdded).UTC()
//
//		if now.Before(executionTime) {
//			executionTransformer := transformers.Execution{}
//			executionTransformer.FromManager(executionManager)
//			recovery := RecoveredJob{
//				Execution: &executionTransformer,
//				Job:       &jobTransformer,
//			}
//			jobProcessor.RecoveredJobs = append(jobProcessor.RecoveredJobs, recovery)
//		}
//	}
//}

// IsRecovered Check if a job is in recovered job queues
func (jobRecovery *JobRecovery) IsRecovered(jobID int64) bool {
	for _, recoveredJob := range jobRecovery.RecoveredJobs {
		if recoveredJob.Job.ID == jobID {
			return true
		}
	}

	return false
}

// GetRecovery Returns recovery object
func (jobRecovery *JobRecovery) GetRecovery(jobID int64) *RecoveredJob {
	for _, recoveredJob := range jobRecovery.RecoveredJobs {
		if recoveredJob.Job.ID == jobID {
			return &recoveredJob
		}
	}

	return nil
}

// RemoveJobRecovery Removes a recovery object
func (jobRecovery *JobRecovery) RemoveJobRecovery(jobID int64) {
	jobIndex := -1

	for index, recoveredJob := range jobRecovery.RecoveredJobs {
		if recoveredJob.Job.ID == jobID {
			jobIndex = index
			break
		}
	}

	jobRecovery.RecoveredJobs = append(jobRecovery.RecoveredJobs[:jobIndex], jobRecovery.RecoveredJobs[jobIndex+1:]...)
}
