package service

import (
	"github.com/victorlenerd/scheduler0/server/src/managers/execution"
	"github.com/victorlenerd/scheduler0/server/src/transformers"
)

type ExecutionService Service

func (executionService *ExecutionService) GetAllExecutionsByJobUUID(jobUUID string, offset int, limit int) ([]transformers.Execution, error) {
	manager := execution.ExecutionManager{}
	executionManagers, err := manager.GetAll(executionService.Pool,  jobUUID, offset, limit, "date_created")
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