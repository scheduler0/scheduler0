package fixtures

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

// CreateNJobTransformers create n number of job transformer fixtures for testing
//func (jobFixture *JobFixture) CreateNJobTransformers(n int) []models.JobModel {
//	jobTransformers := []models.JobModel{}
//
//	for i := 0; i < n; i++ {
//		err := faker.FakeData(&jobFixture)
//		utils.CheckErr(err)
//
//		jobTransformers = append(jobTransformers, models.JobModel{
//			Spec:        "* * * * 1",
//			Data:        jobFixture.Data,
//			CallbackUrl: jobFixture.CallbackUrl,
//		})
//	}
//
//	return jobTransformers
//}
