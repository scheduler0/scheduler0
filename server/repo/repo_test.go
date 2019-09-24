package repo

import (
	"cron-server/server/models"
	"testing"
	"time"
)

func TestJobRepo(t *testing.T) {
	t.Log("Repository")
	{
		var j models.Dto

		t.Logf("\t Create new job")
		{
			j = models.Dto{
				StartDate:   time.Now(),
				ServiceName: "Test",
				CronSpec:    "* * * * *",
				Data:        "some-data",
			}

			d, err := CreateOne(j.ToDomain())

			if err != nil {
				t.Fatalf("\t\t An errro occured: %v", err.Error())
			}

			if len(d.ID) < 1 {
				t.Fatalf("\t\t New job should have id")
			} else {
				j.ID = d.ID
			}
		}

		t.Logf("\t Update job")
		{

			var newNewServiceName string = "new-service-name"
			j.ServiceName = "new-service-name"

			updatedJob, err := UpdateOne(j.ToDomain())

			if err != nil {
				t.Fatalf("\t\t An errro occured: %v", err.Error())
			}

			if updatedJob.ServiceName != newNewServiceName {
				t.Fatalf("\t\t Service name is %v but it should be %v", updatedJob.ServiceName, newNewServiceName)
			}
		}

		t.Logf("\t Delete job")
		{
			if len(j.ID) < 1 {
				t.Fatalf("\t\t Job id is not empty")
			}

			_, err := DeleteOne(j.ID)

			if err != nil {
				t.Fatalf("\t\t An errro occured: %v", err.Error())
			}
		}
	}
}
