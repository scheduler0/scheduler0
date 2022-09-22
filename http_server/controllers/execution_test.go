package controllers_test

//import (
//	"fmt"
//	. "github.com/onsi/ginkgo"
//	. "github.com/onsi/gomega"
//	"net/http"
//	"net/http/httptest"
//	"scheduler0/server/db"
//	"scheduler0/server/http_server/controllers"
//	"scheduler0/server/models"
//	"scheduler0/server/repository/fixtures"
//	"scheduler0/utils"
//	"testing"
//)
//
//var _ = Describe("Execution Controller", func() {
//
//	BeforeEach(func() {
//		db.TeardownTestDB()
//		db.PrepareTestDB()
//	})
//
//	executionsController := controllers.NewExecutionsController()
//	DBConnection := db.GetTestDBConnection()
//
//	It("Get All Returns 0 CountByJobID and Empty Set", func() {
//		Job := fixtures.CreateJobFixture(DBConnection)
//		executionManager := models.ExecutionModel{
//			JobID: Job.ID,
//		}
//
//		_, createOneError := executionsController.CreateOne(executionManager)
//		if createOneError != nil {
//			utils.Error(fmt.Sprintf("Cannot create execution %v", createOneError.Message))
//		}
//
//		req, err := http.NewRequest("GET", fmt.Sprintf("/?jobID=%v&offset=0&limit=10", Job.ID), nil)
//
//		if err != nil {
//			utils.Error(fmt.Sprintf("Cannot create http request %v", err))
//		}
//
//		w := httptest.NewRecorder()
//		executionsController.ListExecutions(w, req)
//
//		Expect(w.Code).To(Equal(http.StatusOK))
//	})
//
//})
//
//func TestExecution_Controller(t *testing.T) {
//	utils.SetTestScheduler0Configurations()
//	RegisterFailHandler(Fail)
//	RunSpecs(t, "Execution Controller Suite")
//}
