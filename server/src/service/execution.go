package service

import (
	"net/http"
	execution "scheduler0/server/src/managers/execution"
	"scheduler0/server/src/transformers"
	"scheduler0/server/src/utils"
)

type ExecutionService Service

func (executionService *ExecutionService) GetAllExecutionsByJobUUID(jobUUID string, offset int, limit int) ([]transformers.Execution, *utils.GenericError) {
	manager := execution.ExecutionManager{}

	count, getCountError := manager.Count(executionService.Pool, jobUUID)
	if getCountError != nil {
		return nil, getCountError
	}

	if count < 1 {
		return nil, utils.HTTPGenericError(http.StatusNotFound, "cannot find executions for file")
	}

	executionManagers, err := manager.List(executionService.Pool, jobUUID, offset, limit, "date_created")
	if err != nil {
		return nil, err
	}

	executions := make([]transformers.Execution, 0, len(executionManagers))

	for _, executionManager := range executionManagers {
		execution := transformers.Execution{}
		execution.FromManager(executionManager)
		executions = append(executions, execution)
	}

	return executions, nil
}
