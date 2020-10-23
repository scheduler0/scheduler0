package managers

import (
	"cron-server/server/src/managers"
	"cron-server/server/src/utils"
	"cron-server/server/tests"
	"fmt"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func createJobFixture(pool *utils.Pool, t *testing.T, JobName string) string {
	projectManager := managers.ProjectManager{
		Name: JobName,
		Description: "some random desc",
	}
	ProjectID, err := projectManager.CreateOne(pool)

	if err != nil {
		t.Fatalf("\t\t [ERROR] failed to create project:: %v", err.Error())
	}

	jobManager := managers.JobManager{
		ProjectID: ProjectID,
		StartDate: time.Now().Add(2000000),
		CallbackUrl: "https://some-random.url",
		CronSpec: "* * * * 1",
	}

	JobID, err := jobManager.CreateOne(pool)
	if err != nil {
		t.Fatalf("\t\t [ERROR] failed to create job %v", err.Error())
	}

	return JobID
}

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
			JobID := createJobFixture(pool, t, "some job name")
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

		JobID := createJobFixture(pool, t, "some job name -- ")

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