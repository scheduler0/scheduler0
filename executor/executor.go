package executor

import (
	"scheduler0/models"
)

type Executor interface {
	ExecuteHTTP() error
}

type Service struct {
	pendingJob           []*models.JobModel
	httpExecutionHandler *HTTPExecutionHandler
	onSuccess            func(pj []*models.JobModel)
	onFailure            func(pj []*models.JobModel, err error)
}

func NewService(pj []*models.JobModel, onSuccess func(pj []*models.JobModel), onFailure func(pj []*models.JobModel, err error)) *Service {
	return &Service{
		pendingJob:           pj,
		httpExecutionHandler: &HTTPExecutionHandler{},
		onSuccess:            onSuccess,
		onFailure:            onFailure,
	}
}

func (executorService *Service) ExecuteHTTP() {
	go func(pjs []*models.JobModel) {
		err := executorService.httpExecutionHandler.ExecuteHTTPJob(pjs)
		if err != nil {
			executorService.onFailure(pjs, err)
			return
		}
		executorService.onSuccess(pjs)
	}(executorService.pendingJob)
}
