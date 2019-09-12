package main

import (
	"cron/controllers"
	"cron/dto"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"time"

	"strings"
	"testing"
)

func TestCron(t *testing.T) {

	t.Log("Register job")
	{
		job := &dto.Job{ServiceName: "test"}

		t.Logf("\t Identify bad request")
		{
			req, err := http.NewRequest("POST", "application/json", strings.NewReader(job.ToJson()))
			w := httptest.NewRecorder()

			if err != nil {
				panic(err)
			}

			controllers.RegisterJob(w, req)

			assert.Equal(t, w.Code, http.StatusBadRequest)
		}

		t.Logf("\t Create new job")
		{
			job.Cron = "1 * * * *"
			job.StartDate = time.Now().UTC()

			req, err := http.NewRequest("POST", "application/json", strings.NewReader(job.ToJson()))
			w := httptest.NewRecorder()

			if err != nil {
				panic(err)
			}

			controllers.RegisterJob(w, req)

			assert.Equal(t, w.Code, http.StatusCreated)
		}
	}
}
