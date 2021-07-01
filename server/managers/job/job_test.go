package job_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"scheduler0/server/db"
	"scheduler0/server/managers/job"
	fixtures2 "scheduler0/server/managers/job/fixtures"
	fixtures3 "scheduler0/server/managers/project/fixtures"
	"scheduler0/utils"
	"testing"
)

var _ = Describe("Job Manager", func() {
	pool := db.GetTestDBConnection()

	BeforeEach(func() {
		db.Teardown()
		db.Prepare()
	})

	Context("JobManager.CreateOne", func() {
		It("Creating job returns error if required inbound fields are nil", func() {
			jobFixture := fixtures2.JobFixture{}
			jobTransformers := jobFixture.CreateNJobTransformers(1)
			jobManager, toManagerError := jobTransformers[0].ToManager()
			if toManagerError != nil {
				utils.Error(toManagerError)
			}

			uuid, err := jobManager.CreateOne(pool)
			if err == nil {
				utils.Error("[ERROR] Model should require values")
			}

			Expect(uuid).To(Equal(""))
		})

		It("Creating job returns new id", func() {
			jobFixture := fixtures2.JobFixture{}
			jobTransformers := jobFixture.CreateNJobTransformers(1)
			jobManager, toManagerError := jobTransformers[0].ToManager()
			if toManagerError != nil {
				utils.Error(toManagerError)
			}
			projectTransformer := fixtures3.CreateProjectTransformerFixture()
			projectManager := projectTransformer.ToManager()
			_, createOneProjectError := projectManager.CreateOne(pool)
			if createOneProjectError != nil {
				utils.Error(fmt.Sprintf("[ERROR] Cannot create project %v", createOneProjectError.Message))
			}

			jobManager.ProjectUUID = projectManager.UUID

			uuid, err := jobManager.CreateOne(pool)
			if err != nil {
				utils.Error(fmt.Sprintf("[ERROR] Cannot create job %v", err.Message))
			}

			if len(uuid) < 1 {
				utils.Error(fmt.Sprintf("[ERROR] Project uuid is invalid %v", uuid))
			}
		})
	})

	Context("JobManager.UpdateOne", func() {
		It("Cannot update cron spec on job", func() {
			jobFixture := fixtures2.JobFixture{}
			jobTransformers := jobFixture.CreateNJobTransformers(1)
			jobManager, toManagerError := jobTransformers[0].ToManager()
			if toManagerError != nil {
				utils.Error(toManagerError)
			}
			projectTransformer := fixtures3.CreateProjectTransformerFixture()
			projectManager := projectTransformer.ToManager()
			_, createOneProjectError := projectManager.CreateOne(pool)
			if createOneProjectError != nil {
				utils.Error(fmt.Sprintf("[ERROR] Cannot create project %v", createOneProjectError.Message))
			}

			jobManager.ProjectUUID = projectManager.UUID

			uuid, err := jobManager.CreateOne(pool)
			if err != nil {
				utils.Error(fmt.Sprintf("[ERROR] Cannot create job %v", err.Message))
			}

			jobGetManager := job.Manager{
				UUID: uuid,
			}

			jobGetManager.ProjectUUID = projectManager.UUID
			jobGetManager.Spec = "1 * * * *"

			_, updateOneError := jobGetManager.UpdateOne(pool)
			if updateOneError == nil {
				utils.Error("[ERROR] Job cron spec should not be replaced")
			}
		})
	})

	Context("JobManager.DeleteOne", func() {
		It("Delete jobs", func() {
			jobFixture := fixtures2.JobFixture{}
			jobTransformers := jobFixture.CreateNJobTransformers(1)
			jobManager, toManagerError := jobTransformers[0].ToManager()
			if toManagerError != nil {
				utils.Error(toManagerError)
			}
			projectTransformer := fixtures3.CreateProjectTransformerFixture()
			projectManager := projectTransformer.ToManager()
			_, createOneProjectError := projectManager.CreateOne(pool)
			if createOneProjectError != nil {
				utils.Error(fmt.Sprintf("[ERROR] Cannot create project %v", createOneProjectError.Message))
			}

			jobManager.ProjectUUID = projectManager.UUID
			jobManager.CreateOne(pool)

			rowsAffected, err := jobManager.DeleteOne(pool)
			if err != nil && rowsAffected > 0 {
				utils.Error(err.Message)
			}
		})
	})

	It("JobManager.List", func() {
		jobFixture := fixtures2.JobFixture{}
		jobTransformers := jobFixture.CreateNJobTransformers(5)

		projectTransformer := fixtures3.CreateProjectTransformerFixture()
		projectManager := projectTransformer.ToManager()
		_, createOneProjectError := projectManager.CreateOne(pool)
		if createOneProjectError != nil {
			utils.Error(fmt.Sprintf("[ERROR] Cannot create project %v", createOneProjectError.Message))
		}

		for i := 0; i < 5; i++ {
			jobManager, toManagerError := jobTransformers[i].ToManager()
			if toManagerError != nil {
				utils.Error(toManagerError)
			}
			jobManager.ProjectID = projectManager.ID
			jobManager.ProjectUUID = projectManager.UUID
			_, createOneJobManagerError := jobManager.CreateOne(pool)
			if createOneJobManagerError != nil {
				utils.Error(toManagerError)
			}
		}

		jobGetManager := job.Manager{}
		jobs, _, getAllJobsError := jobGetManager.GetJobsPaginated(pool, projectManager.UUID, 0, 5)
		if getAllJobsError != nil {
			utils.Error(fmt.Sprintf("[ERROR] Cannot get all projects %v", getAllJobsError.Message))
		}

		Expect(len(jobs)).To(Equal(5))
	})

	It("JobManager.GetOne", func() {
		jobFixture := fixtures2.JobFixture{}
		jobTransformers := jobFixture.CreateNJobTransformers(1)
		jobManager, toManagerError := jobTransformers[0].ToManager()
		if toManagerError != nil {
			utils.Error(toManagerError)
		}
		projectTransformer := fixtures3.CreateProjectTransformerFixture()
		projectManager := projectTransformer.ToManager()
		_, createOneProjectError := projectManager.CreateOne(pool)
		if createOneProjectError != nil {
			utils.Error(fmt.Sprintf("[ERROR] Cannot create project %v", createOneProjectError.Message))
		}
		jobManager.ProjectUUID = projectManager.UUID
		jobManager.CreateOne(pool)
		jobResult := job.Manager{}
		getOneJobError := jobResult.GetOne(pool, jobManager.UUID)
		if getOneJobError != nil {
			utils.Error(fmt.Sprintf("[ERROR]  Failed to get job by id %v", getOneJobError.Message))
		}

		Expect(jobResult.ProjectUUID).To(Equal(jobManager.ProjectUUID))
	})
})

func TestJob_Manager(t *testing.T) {
	utils.SetTestScheduler0Configurations()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Job Manager Suite")
}
