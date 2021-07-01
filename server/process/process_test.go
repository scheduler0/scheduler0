package process

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robfig/cron"
	"scheduler0/server/db"
	"scheduler0/server/transformers"
	"testing"
)

var _ = Describe("Job Processor", func() {

	It("Should return true if job is in recovered list", func () {
		pool := db.GetTestDBConnection()

		processor := JobProcessor{
			RecoveredJobs: []RecoveredJob{},
			DBConnection: pool,
			Cron: cron.New(),
		}

		processor.RecoveredJobs = append(processor.RecoveredJobs, RecoveredJob{
			Job: &transformers.Job{
				UUID: "some-random-uuid",
			},
		})

		isRecoveredJob := processor.IsRecovered("some-random-uuid")

		Expect(isRecoveredJob).To(Equal(true))
	})

	It("Should return false if job is not in recovered list", func () {
		pool := db.GetTestDBConnection()

		processor := JobProcessor{
			RecoveredJobs: []RecoveredJob{},
			DBConnection: pool,
			Cron: cron.New(),
		}

		processor.RecoveredJobs = append(processor.RecoveredJobs, RecoveredJob{
			Job: &transformers.Job{
				UUID: "some-random-uuid",
			},
		})

		isRecoveredJob := processor.IsRecovered("not-so-some-random-uuid")

		Expect(isRecoveredJob).To(Equal(false))
	})

})

func Test_JobProcessor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Job Processor")
}