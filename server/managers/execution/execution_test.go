package execution_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"scheduler0/server/db"
	"scheduler0/server/managers/execution"
	fixtures "scheduler0/server/managers/execution/fixtures"
	"scheduler0/utils"
	"strconv"
	"testing"
	"time"
)

func TestManager_Executions(t *testing.T) {
	utils.SetTestScheduler0Configurations()
	dbConnection := db.GetTestDBConnection()

	t.Run("Batch Insertion", func(t *testing.T) {
		jobManager := fixtures.CreateJobFixture(dbConnection)
		executionManagers := []execution.Manager{}
		executionManager := execution.Manager{}

		for i := 0; i < 3; i++ {
			executionManagers = append(executionManagers, execution.Manager{
				JobID:       jobManager.ID,
				JobUUID:     jobManager.UUID,
				TimeAdded:   time.Now().UTC(),
				DateCreated: time.Now().UTC(),
			})
		}

		uuids, err := executionManager.BatchInsertExecutions(dbConnection, executionManagers)
		if err != nil {
			t.Error(err)
		}

		fmt.Println(uuids)
		assert.Equal(t, len(uuids), 3)
	})

	t.Run("Batch Update", func(t *testing.T) {
		jobManager := fixtures.CreateJobFixture(dbConnection)
		executionManagers := []execution.Manager{}
		executionManager := execution.Manager{}

		for i := 0; i < 3; i++ {
			executionManagers = append(executionManagers, execution.Manager{
				JobID:       jobManager.ID,
				JobUUID:     jobManager.UUID,
				TimeAdded:   time.Now().UTC(),
				DateCreated: time.Now().UTC(),
			})
		}

		uuids, err := executionManager.BatchInsertExecutions(dbConnection, executionManagers)
		if err != nil {
			t.Error(err)
		}

		for i, _ := range executionManagers {
			executionManagers[i].UUID = uuids[i]
			executionManagers[i].ExecutionTime = 100 + int64(i)
			executionManagers[i].TimeExecuted = time.Now().UTC()
			executionManagers[i].StatusCode = strconv.Itoa(200 + int(i))
		}

		err = executionManager.BatchUpdateExecutions(dbConnection, executionManagers)
		assert.Nil(t, err)
	})

	t.Run("Do not create execution without job id", func(t *testing.T) {
		executionManager := execution.Manager{}
		_, err := executionManager.CreateOne(dbConnection)
		assert.NotEqual(t, err, nil)
	})

	t.Run("Create execution with valid job id", func(t *testing.T) {
		jobManager := fixtures.CreateJobFixture(dbConnection)
		executionManager := execution.Manager{
			JobID:   jobManager.ID,
			JobUUID: jobManager.UUID,
		}
		_, err := executionManager.CreateOne(dbConnection)
		assert.Nil(t, err)
	})

	t.Run("Returns 0 if execution does not exist", func(t *testing.T) {
		executionManager := execution.Manager{UUID: "some-random-id"}
		count, err := executionManager.GetOne(dbConnection)
		assert.Nil(t, err)
		assert.Equal(t, count == 0, true)
	})

	t.Run("Returns count 1 if execution exist", func(t *testing.T) {
		jobManager := fixtures.CreateJobFixture(dbConnection)
		executionManager := execution.Manager{
			JobID:   jobManager.ID,
			JobUUID: jobManager.UUID,
		}
		executionManagerUUID, err := executionManager.CreateOne(dbConnection)
		assert.Nil(t, err)

		executionManager = execution.Manager{UUID: executionManagerUUID}
		count, err := executionManager.GetOne(dbConnection)
		assert.Nil(t, err)

		assert.Equal(t, count > 0, true)
	})

	t.Run("Paginated results from manager", func(t *testing.T) {
		jobManager := fixtures.CreateJobFixture(dbConnection)

		for i := 0; i < 1000; i++ {
			executionManager := execution.Manager{
				JobID:   jobManager.ID,
				JobUUID: jobManager.UUID,
			}

			_, err := executionManager.CreateOne(dbConnection)

			assert.Nil(t, err)
			if err != nil {
				utils.Error(err.Message)
			}
		}

		manager := execution.Manager{}

		executions, err := manager.List(dbConnection, jobManager.UUID, 0, 100, "date_created")
		if err != nil {
			utils.Error(fmt.Sprintf("[ERROR] fetching executions %v", err.Message))
		}

		assert.Equal(t, len(executions), 100)

		executions, err = manager.List(dbConnection, jobManager.UUID, 1000, 100, "date_created")
		if err != nil {
			utils.Error(fmt.Sprintf("[ERROR] fetching executions %v", err.Message))
		}

		assert.Equal(t, len(executions), 0)
	})
}
