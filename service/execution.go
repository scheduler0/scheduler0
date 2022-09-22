package service

import (
	"net/http"
	"scheduler0/models"
	repos "scheduler0/repository"
	"scheduler0/utils"
)

// ExecutionService performs main business logic for executions
type ExecutionService interface {
	GetAllExecutionsByJobUUID(jobID int64, offset int64, limit int64) (*models.PaginatedExecution, *utils.GenericError)
}

type executionService struct {
	executionRepo repos.Execution
}

func NewExecutionService(executionRepo repos.Execution) ExecutionService {
	return &executionService{
		executionRepo: executionRepo,
	}
}

// GetAllExecutionsByJobUUID returns a paginated executions result set
func (executionService *executionService) GetAllExecutionsByJobUUID(jobID int64, offset int64, limit int64) (*models.PaginatedExecution, *utils.GenericError) {
	count, getCountError := executionService.executionRepo.CountByJobID(jobID)
	if getCountError != nil {
		return nil, getCountError
	}

	if count < 1 {
		return nil, utils.HTTPGenericError(http.StatusNotFound, "cannot find executions for file")
	}

	executionManagers, err := executionService.executionRepo.ListByJobID(jobID, offset, limit, "date_created")
	if err != nil {
		return nil, err
	}

	paginatedExecutions := models.PaginatedExecution{}
	paginatedExecutions.Data = executionManagers
	paginatedExecutions.Total = int64(count)
	paginatedExecutions.Offset = offset
	paginatedExecutions.Limit = limit

	return &paginatedExecutions, nil
}
