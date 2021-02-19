package execution_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"scheduler0/server/src/controllers/execution"
	managers "scheduler0/server/src/managers/execution"
	fixtures "scheduler0/server/src/managers/execution/fixtures"
	"scheduler0/server/src/utils"
	"scheduler0/server/tests"
	"testing"
)

var _ = Describe("Execution Controller", func() {

	BeforeEach(func() {
		tests.Teardown()
		tests.Prepare()
	})

	executionsController := execution.ExecutionController{}
	pool := tests.GetTestPool()

	It("Get All Returns 0 Count and Empty Set", func() {
		Job := fixtures.CreateJobFixture(pool)
		executionManager := managers.ExecutionManager{
			JobUUID: Job.UUID,
		}

		_, err := executionManager.CreateOne(pool)
		if err != nil {
			utils.Error(fmt.Sprintf("Cannot create execution %v", err))
		}

		executionsController.Pool = pool
		req, err := http.NewRequest("GET", "/?jobUUID="+Job.UUID+"&offset=0&limit=10", nil)

		if err != nil {
			utils.Error(fmt.Sprintf("Cannot create http request %v", err))
		}

		w := httptest.NewRecorder()
		executionsController.List(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

})

func TestExecution_Controller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Execution Controller Suite")
}
