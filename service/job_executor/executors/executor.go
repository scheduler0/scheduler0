package executors

import (
	"golang.org/x/net/context"
	"log"
	"scheduler0/models"
)

type Executor interface {
	ExecuteHTTP() error
}

type Service struct {
	httpExecutionHandler *HTTPExecutionHandler
}

func NewService(logger *log.Logger) *Service {
	return &Service{
		httpExecutionHandler: NewHTTTPExecutor(logger),
	}
}

func (executorService *Service) ExecuteHTTP(pj []models.JobModel, ctx context.Context, onSuccess func(pj []models.JobModel), onFailure func(pj []models.JobModel)) {
	go func(pjs []models.JobModel) {
		executorService.httpExecutionHandler.ExecuteHTTPJob(ctx, pjs, onSuccess, onFailure)
	}(pj)
}
