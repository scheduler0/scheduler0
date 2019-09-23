package main

import (
	"cron-server/server/controllers"
	"cron-server/server/job"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCron(t *testing.T) {

	var j *job.Dto

	t.Log("Register and Activate job")
	{
		j = &job.Dto{ServiceName: "test"}

		t.Logf("\t Identify bad request")
		{
			req, err := http.NewRequest("POST", "application/json", strings.NewReader(j.ToJson()))
			w := httptest.NewRecorder()

			if err != nil {
				panic(err)
			}

			controllers.RegisterJob(w, req)

			assert.Equal(t, w.Code, http.StatusBadRequest)
		}

		t.Logf("\t Create new job")
		{
			j.CronSpec = "1 * * * *"
			j.StartDate = time.Now().UTC()

			req, err := http.NewRequest("POST", "application/json", strings.NewReader(j.ToJson()))
			w := httptest.NewRecorder()

			if err != nil {
				panic(err)
			}

			controllers.RegisterJob(w, req)

			locationHeader := w.Header().Get("Location")

			if len(locationHeader) < 1 {
				t.Fatalf("\t\t The location header in response is empty :: %v", locationHeader)
			}

			j.ID = strings.Split(locationHeader, "/")[1]

			assert.Equal(t, w.Code, http.StatusCreated)
		}

		t.Logf("\t Activate job")
		{
			req, err := http.NewRequest("PUT", "application/json", nil)
			w := httptest.NewRecorder()

			req.URL.Host = "https"
			req.URL.Path = "/activate/" + j.ID

			if err != nil {
				panic(err)
			}

			time.Sleep(time.Second * 10)
			controllers.ActivateJob(w, req)
			assert.Equal(t, w.Code, http.StatusOK)
		}
	}
}
