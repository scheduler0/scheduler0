package process

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robfig/cron"
	"scheduler0/db"
	"scheduler0/server/transformers"
	"testing"
)

var _ = Describe("Job Processor", func() {

	It("Should return true if job is in recovered list", func() {
		pool := db.GetTestDBConnection()

		processor := JobProcessor{
			RecoveredJobs: []RecoveredJob{},
			DBConnection:  pool,
			Cron:          cron.New(),
		}

		processor.RecoveredJobs = append(processor.RecoveredJobs, RecoveredJob{
			Job: &transformers.Job{
				ID: 1,
			},
		})

		isRecoveredJob := processor.IsRecovered(1)

		Expect(isRecoveredJob).To(Equal(true))
	})

	It("Should return false if job is not in recovered list", func() {
		pool := db.GetTestDBConnection()

		processor := JobProcessor{
			RecoveredJobs: []RecoveredJob{},
			DBConnection:  pool,
			Cron:          cron.New(),
		}

		processor.RecoveredJobs = append(processor.RecoveredJobs, RecoveredJob{
			Job: &transformers.Job{
				ID: 2,
			},
		})

		isRecoveredJob := processor.IsRecovered(1)

		Expect(isRecoveredJob).To(Equal(false))
	})

})

func Test_JobProcessor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Job Processor")
}
