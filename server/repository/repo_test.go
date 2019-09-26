package repository

import (
	"cron-server/server/models"
	"testing"
	"time"
)

func TestJobRepo(t *testing.T) {
	t.Log("Repository")
	{
		var j models.Job

		t.Logf("\t Create new job")
		{
			j = models.Job{
				StartDate: time.Now(),
				ProjectId: "Test",
				CronSpec:  "* * * * *",
				Data:      "some-data",
			}

			id, err := j.CreateOne()

			if err != nil {
				t.Fatalf("\t\t An errro occured: %v", err.Error())
			}

			if len(id) < 1 {
				t.Fatalf("\t\t New job should have id")
			} else {
				j.ID = id
			}
		}

		t.Logf("\t Update job")
		{
			j.ProjectId = "new-service-name"

			err := j.UpdateOne()

			if err != nil {
				t.Fatalf("\t\t An errro occured: %v", err.Error())
			}
		}

		t.Logf("\t Delete job")
		{
			if len(j.ID) < 1 {
				t.Fatalf("\t\t Job id is not empty")
			}

			err := j.DeleteOne()

			if err != nil {
				t.Fatalf("\t\t An errro occured: %v", err.Error())
			}
		}
	}
}
