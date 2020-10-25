package service

import (
	"cron-server/server/src/managers"
	"cron-server/server/src/transformers"
)

type JobService Service

func (jobService *JobService) GetJobsByProjectID(projectID string, offset int, limit int, orderBy string) ([]transformers.Job, error) {
	jobManager := managers.JobManager{}

	jobManagers, err := jobManager.GetAll(jobService.Pool, projectID, offset, limit, orderBy)
	if err != nil {
		return nil, err
	}

	jobs := make([]transformers.Job, 0, len(jobManagers))

	for _, jobManager := range jobManagers {
		jobsTransformer := transformers.Job{}
		jobsTransformer.FromManager(jobManager)
		jobs = append(jobs, jobsTransformer)
	}

	return jobs, nil
}