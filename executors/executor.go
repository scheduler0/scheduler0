package executors

import (
	"log"
	"scheduler0/models"
)

type Executor interface {
	ExecuteHTTP() error
}

type Service struct {
	pendingJob           []models.JobModel
	httpExecutionHandler *HTTPExecutionHandler
	onSuccess            func(pj []models.JobModel)
	onFailure            func(pj []models.JobModel)
}

func NewService(logger *log.Logger, pj []models.JobModel, onSuccess func(pj []models.JobModel), onFailure func(pj []models.JobModel)) *Service {
	return &Service{
		pendingJob:           pj,
		httpExecutionHandler: NewHTTTPExecutor(logger),
		onSuccess:            onSuccess,
		onFailure:            onFailure,
	}
}

func (executorService *Service) ExecuteHTTP() {
	go func(pjs []models.JobModel) {
		executorService.httpExecutionHandler.ExecuteHTTPJob(pjs, executorService.onSuccess, executorService.onFailure)
	}(executorService.pendingJob)
}
