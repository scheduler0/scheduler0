package fixtures

import (
	"github.com/bxcodec/faker/v3"
	"log"
	"scheduler0/models"
)

// JobFixture job fixture for testing
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

// CreateNJobModels create n number of job transformer fixtures for testing
func (jobFixture *JobFixture) CreateNJobModels(n int) []models.JobModel {
	jobModels := []models.JobModel{}

	for i := 0; i < n; i++ {
		err := faker.FakeData(&jobFixture)
		if err != nil {
			log.Fatal("failed to create faker", err)
		}

		jobModels = append(jobModels, models.JobModel{
			Spec:        "* * * * 1",
			Data:        jobFixture.Data,
			CallbackUrl: jobFixture.CallbackUrl,
		})
	}

	return jobModels
}
