package service

import (
	"net/http"
	execution "scheduler0/server/managers/execution"
	"scheduler0/server/transformers"
	"scheduler0/utils"
)

// ExecutionService performs main business logic for executions
type ExecutionService Service

// GetAllExecutionsByJobUUID returns a paginated executions result set
func (executionService *ExecutionService) GetAllExecutionsByJobUUID(jobUUID string, offset int, limit int) (*transformers.PaginatedExecution, *utils.GenericError) {
	manager := execution.Manager{}

	count, getCountError := manager.Count(executionService.DBConnection, jobUUID)
	if getCountError != nil {
		return nil, getCountError
	}

	if count < 1 {
		return nil, utils.HTTPGenericError(http.StatusNotFound, "cannot find executions for file")
	}

	executionManagers, err := manager.List(executionService.DBConnection, jobUUID, offset, limit, "date_created")
	if err != nil {
		return nil, err
	}

	executions := make([]transformers.Execution, 0, len(executionManagers))

	for _, executionManager := range executionManagers {
		executionTransformer := transformers.Execution{}
		executionTransformer.FromManager(executionManager)
		executions = append(executions, executionTransformer)
	}

	paginatedExecutions := transformers.PaginatedExecution{}
	paginatedExecutions.Data = executions
	paginatedExecutions.Total = count
	paginatedExecutions.Offset = offset
	paginatedExecutions.Limit = limit

	return &paginatedExecutions, nil
}
