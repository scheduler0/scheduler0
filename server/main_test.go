package main

import (
	//"cron-server/server/controllers/job"
	//"cron-server/server/misc"
	"cron-server/server/models"
	//"github.com/stretchr/testify/assert"
	//"net/http"
	//"net/http/httptest"
	//"strings"
	"testing"
	//"time"
)

func TestCron(t *testing.T) {

	var j *models.Job

	t.Log("Register and Activate job")
	{
		j = &models.Job{ProjectId: "test"}

		t.Logf("\t Identify bad request")
		{
			// TODO: Test that bad request can be identified
		}

		t.Logf("\t Create new job")
		{
			// TODO: Test that job can be created
		}

		t.Logf("\t Activate job")
		{
			// TODO: Test that job can be activated
		}
	}
}
