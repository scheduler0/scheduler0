package controllers

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/victorlenerd/scheduler0/server/src/controllers"
	"github.com/victorlenerd/scheduler0/server/src/managers"
	"github.com/victorlenerd/scheduler0/server/tests"
	"github.com/victorlenerd/scheduler0/server/tests/fixtures"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecutionController_GetAll(t *testing.T) {
	executionsController := controllers.ExecutionController{}
	pool := tests.GetTestPool()

	t.Log("Get All Returns 0 Count and Empty Set")
	{
		Job := fixtures.CreateJobFixture(pool, t)
		executionManager := managers.ExecutionManager{
			JobID: Job.ID,
		}

		_, err := executionManager.CreateOne(pool)
		if err != nil {
			t.Fatalf("\t\t Cannot create execution %v", err)
		}

		executionsController.Pool = pool
		req, err := http.NewRequest("GET", "/?jobID="+Job.ID+"&offset=0&limit=10", nil)

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
