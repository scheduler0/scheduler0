package execution_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"scheduler0/server/db"
	"scheduler0/server/http_server/controllers/execution"
	managers "scheduler0/server/managers/execution"
	fixtures "scheduler0/server/managers/execution/fixtures"
	"scheduler0/utils"
	"testing"
)

var _ = Describe("Execution Controller", func() {

	BeforeEach(func() {
		db.Teardown()
		db.Prepare()
	})

	executionsController := execution.Controller{}
	pool := db.GetTestPool()

	It("Get All Returns 0 Count and Empty Set", func() {
		Job := fixtures.CreateJobFixture(pool)
		executionManager := managers.Manager{
			JobUUID: Job.UUID,
		}

		_, createOneError := executionManager.CreateOne(pool)
		if createOneError != nil {
			utils.Error(fmt.Sprintf("Cannot create execution %v", createOneError.Message))
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
	utils.SetTestScheduler0Configurations()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Execution Controller Suite")
}
