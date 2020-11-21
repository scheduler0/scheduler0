package controllers

import (
	"cron-server/server/src/controllers"
	"cron-server/server/src/managers"
	"cron-server/server/tests"
	"cron-server/server/tests/fixtures"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecutionController_GetAll(t *testing.T) {
	executionsController := controllers.ExecutionController{}
	pool := tests.GetTestPool()

	t.Log("Get All Returns 0 Count and Empty Set")
	{
		JobID := fixtures.CreateJobFixture(pool, t)
		executionManager := managers.ExecutionManager{
			JobID: JobID,
		}

		_, err := executionManager.CreateOne(pool)
		if err != nil {
			t.Fatalf("\t\t Cannot create execution %v", err)
		}

		executionsController.Pool = pool
		req, err := http.NewRequest("GET", "/?jobID="+JobID+"&offset=0&limit=10", nil)

		if err != nil {
			t.Fatalf("\t\t Cannot create http request %v", err)
		}

		w := httptest.NewRecorder()
		executionsController.List(w, req)

		res, _, err := tests.ExtractResponse(w)
		if err != nil {
			t.Fatalf("\t\t json error %v", err)
		}

		fmt.Println(res)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}
