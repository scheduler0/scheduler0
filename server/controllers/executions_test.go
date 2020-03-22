package controllers

import (
	"cron-server/server/migrations"
	"cron-server/server/misc"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

var (
	executionsController = ExecutionController{}
)

func TestExecutionController_GetAll(t *testing.T) {
	t.Log("Get All Returns 0 Count and Empty Set")
	{
		var pool, err = migrations.NewPool(migrations.CreateConnection, 1)
		misc.CheckErr(err)
		executionsController.Pool = *pool

		req, err := http.NewRequest("GET", "/?offset=0&limit=10", nil)

		if err != nil {
			t.Fatalf("\t\t Cannot create http request %v", err)
		}

		w := httptest.NewRecorder()
		executionsController.GetAll(w, req)

		body, err := ioutil.ReadAll(w.Body)
		if err != nil {
			fmt.Print(err)
		}

		var res misc.Response

		err = json.Unmarshal(body, res)
		if err != nil {
			fmt.Print(err)
		}

		assert.Equal(t, http.StatusOK, w.Code)
	}
}
