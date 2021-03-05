package execution_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"scheduler0/server/db"
	"scheduler0/server/managers/execution"
	fixtures "scheduler0/server/managers/execution/fixtures"
	"scheduler0/utils"
	"testing"
)

var _ = Describe("Execution Manager", func() {
	pool := db.GetTestPool()

	db.Teardown()
	db.Prepare()

	It("Do not create execution without job id", func() {
		db.Teardown()
		db.Prepare()

		executionManager := execution.Manager{}
		_, err := executionManager.CreateOne(pool)
		Expect(err).ToNot(BeNil())
	})

	It("Create execution with valid job id", func() {
		db.Teardown()
		db.Prepare()

		jobManager := fixtures.CreateJobFixture(pool)
		executionManager := execution.Manager{
			JobID:   jobManager.ID,
			JobUUID: jobManager.UUID,
		}
		_, err := executionManager.CreateOne(pool)
		Expect(err).To(BeNil())
	})

	It("Returns 0 if execution does not exist", func() {
		db.Teardown()
		db.Prepare()

		executionManager := execution.Manager{UUID: "some-random-id"}
		count, err := executionManager.GetOne(pool)
		Expect(err).To(BeNil())
		Expect(count == 0).To(BeTrue())
	})

	It("Returns count 1 if execution exist", func() {
		db.Teardown()
		db.Prepare()

		jobManager := fixtures.CreateJobFixture(pool)
		executionManager := execution.Manager{
			JobID:   jobManager.ID,
			JobUUID: jobManager.UUID,
		}
		executionManagerUUID, err := executionManager.CreateOne(pool)
		Expect(err).To(BeNil())

		executionManager = execution.Manager{UUID: executionManagerUUID}
		count, err := executionManager.GetOne(pool)
		Expect(err).To(BeNil())

		Expect(count > 0).To(BeTrue())
	})

	It("Paginated results from manager", func() {
		db.Teardown()
		db.Prepare()

		jobManager := fixtures.CreateJobFixture(pool)

		for i := 0; i < 1000; i++ {
			executionManager := execution.Manager{
				JobID:   jobManager.ID,
				JobUUID: jobManager.UUID,
			}

			_, err := executionManager.CreateOne(pool)

			Expect(err).To(BeNil())
			if err != nil {
				utils.Error(err.Message)
			}
		}

		manager := execution.Manager{}

		executions, err := manager.List(pool, jobManager.UUID, 0, 100, "date_created")
		if err != nil {
			utils.Error(fmt.Sprintf("[ERROR] fetching executions %v", err.Message))
		}

		Expect(len(executions)).To(Equal(100))

		executions, err = manager.List(pool, jobManager.UUID, 1000, 100, "date_created")
		if err != nil {
			utils.Error(fmt.Sprintf("[ERROR] fetching executions %v", err.Message))
		}

		Expect(len(executions)).To(Equal(0))
	})

})

func TestExecution_Manager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Execution Manager Suite")
}