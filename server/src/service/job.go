package service

import (
	"cron-server/server/src/managers"
	"cron-server/server/src/transformers"
	"errors"
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

func (jobService *JobService) GetJob(job transformers.Job) (*transformers.Job, error) {
	jobManager, err := job.ToManager()
	if err != nil {
		return nil, err
	}

	err = jobManager.GetOne(jobService.Pool, job.ID)
	if err != nil {
		return nil, err
	}

	job.FromManager(jobManager)

	return &job, nil
}

func (jobService *JobService) CreateJob(job transformers.Job) (*transformers.Job, error) {
	jobManager, err := job.ToManager()
	if err != nil {
		return nil, err
	}

	_, err = jobManager.CreateOne(jobService.Pool)
	if err != nil {
		return nil, err
	}

	job.FromManager(jobManager)

	// TODO: Start go routine for job

	return &job, nil
}

func (jobService *JobService) UpdateJob(job transformers.Job) (*transformers.Job, error) {
	jobManager, err := job.ToManager()
	if err != nil {
		return nil, err
	}

	_, err = jobManager.UpdateOne(jobService.Pool)
	if err != nil {
		return nil, err
	}

	job.FromManager(jobManager)

	return &job, nil
}

func (jobService *JobService) DeleteJob(job transformers.Job) error {
	jobManager := managers.JobManager{}
	count, err := jobManager.DeleteOne(jobService.Pool, job.ID)
	if err != nil {
		return err
	}

	if count < 1 {
		return errors.New("could not find and delete job")
	}

	return nil
}



