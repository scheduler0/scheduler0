package managers

import (
	"cron-server/server/src/managers"
	"cron-server/server/tests"
	"cron-server/server/tests/fixtures"
	"fmt"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_ExecutionManager(t *testing.T) {
	pool := tests.GetTestPool()
	getOneTestExecutionID := ""

	t.Log("ExecutionManager.CreateOne")
	{
		t.Logf("\t\t Do not create execution without job id")
		{
			executionManager := managers.ExecutionManager{}
			_, err := executionManager.CreateOne(pool)

			if err == nil {
				t.Fatal("\t\t [ERROR] creating execution without job id")
			}
		}

		t.Logf("\t\t Do not create execution without valid job id")
		{
			executionManager := managers.ExecutionManager{}

			executionManager.JobID = ksuid.New().String()

			_, err := executionManager.CreateOne(pool)

			if err == nil {
				t.Fatal("\t\t [ERROR] creating execution without valid job id")
			}
		}

		t.Logf("\t\t Create execution with valid job id")
		{
			JobID := fixtures.CreateJobFixture(pool, t)
			executionManager := managers.ExecutionManager{
				JobID: JobID,
			}

			_, err := executionManager.CreateOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] failed to create execution %v", err.Error())
			}

			getOneTestExecutionID = executionManager.ID
		}
	}

	t.Logf("ExecutionManager.GetOne")
	{
		t.Logf("\t\t Returns 0 if execution does not exist")
		{
			executionManager := managers.ExecutionManager{ID: "some-random-id"}
			count, err := executionManager.GetOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] failed to get execution: %v", err.Error())
			}

			if count > 0 {
				t.Fatalf("\t\t [ERROR] should not return any execution: count is %v", count)
			}
		}

		t.Logf("\t\t Returns count 1 if execution exist")
		{
			executionManager := managers.ExecutionManager{ID: getOneTestExecutionID}
			count, err := executionManager.GetOne(pool)
			if err != nil {
				t.Fatalf("\t\t \t\t [ERROR] failed to get execution: %v", err.Error())
			}

			if count < 1 {
				t.Fatalf("\t\t \t\t [ERROR] should return an execution: count is %v", count)
			}

			fmt.Println(executionManager)
		}

		JobID := fixtures.CreateJobFixture(pool, t)

		t.Logf("\t\t Paginated results from manager")
		{
			for i := 0; i < 1000; i++ {
				executionManager := managers.ExecutionManager{
					JobID: JobID,
				}

				_, err := executionManager.CreateOne(pool)

				if err != nil {
					t.Fatalf("\t\t [ERROR] failed to create execution %v", err.Error())
				}
			}

			manager := managers.ExecutionManager{}

			executions, err := manager.GetAll(pool, JobID, 0, 100, "date_created")
			if err != nil {
				t.Fatalf("\t\t [ERROR] fetching executions %v", err.Error())
			}

			assert.Equal(t, 100, len(executions))

			executions, err = manager.GetAll(pool, JobID, 1000, 100, "date_created")
			if err != nil {
				t.Fatalf("\t\t [ERROR] fetching executions %v", err.Error())
			}

			assert.Equal(t, 0, len(executions))
		}
	}
}