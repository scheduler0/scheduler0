package fixtures

import (
	"github.com/bxcodec/faker/v3"
	"github.com/victorlenerd/scheduler0/server/src/transformers"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"time"
)

type JobFixture struct {
	UUID        string `faker:"uuid_hyphenated"`
	ProjectUUID string `faker:"uuid_hyphenated"`
	Description string `faker:"sentence"`
	CronSpec    string
	Data        string `faker:"username"`
	Timezone    string `faker:"timezone"`
	CallbackUrl string `faker:"ipv6"`
	StartDate   string `faker:"timestamp"`
	EndDate     string `faker:"timestamp"`
}


func (jobFixture *JobFixture) CreateNJobTransformers(n int) []transformers.Job {
	jobTransformers := []transformers.Job{}

	for i := 0; i < n; i ++ {
		err := faker.FakeData(&jobFixture)
		utils.CheckErr(err)

		startDate := time.Now().Add(99999999000000000).UTC().Format(time.RFC3339)

		jobTransformers = append(jobTransformers, transformers.Job{
			UUID: jobFixture.UUID,
			ProjectUUID: "",
			Description: jobFixture.Description,
			CronSpec: "* * * * 1",
			Data: jobFixture.Data,
			Timezone: jobFixture.Timezone,
			CallbackUrl: jobFixture.CallbackUrl,
			StartDate: startDate,
			EndDate: "",
		})
	}

	return jobTransformers
}