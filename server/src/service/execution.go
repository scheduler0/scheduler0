package service

import (
	"cron-server/server/src/managers"
	"cron-server/server/src/transformers"
)

type ExecutionService Service

func (executionService *ExecutionService) GetAllExecutionsByJobID(jobID string, offset int, limit int) ([]transformers.Execution, error) {
	manager := managers.ExecutionManager{}
	executionManagers, err := manager.GetAll(executionService.Pool,  jobID, offset, limit, "date_created")
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