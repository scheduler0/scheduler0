package controllers_test

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/http_server/controllers"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/service"
	"scheduler0/utils"
	"strings"
	"testing"
)

var _ = Describe("Credential Controller", func() {

	BeforeEach(func() {
		db.TeardownTestDB()
		db.PrepareTestDB()
	})

	DBConnection := db.GetTestDBConnection()
	store := fsm.NewFSMStore(nil, DBConnection)
	credentialRepo := repository.NewCredentialRepo(store)
	ctx := context.Background()
	credentialService := service.NewCredentialService(credentialRepo, ctx)
	credentialController := controllers.NewCredentialController(credentialService)

	It("Creating A New Credential", func() {
		testCredential := models.CredentialModel{
			HTTPReferrerRestriction: "*",
			Platform:                "web",
		}
		jsonTestCredentialBody, err := testCredential.ToJSON()
		if err != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", err))
		}
		Expect(err).To(BeNil())

		req, err := http.NewRequest("POST", "/", strings.NewReader(string(jsonTestCredentialBody)))
		if err != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", err))
		}
		Expect(err).To(BeNil())

		w := httptest.NewRecorder()
		credentialController.CreateOneCredential(w, req)

		Expect(w.Code).To(Equal(http.StatusCreated))
	})

	It("Get All Credentials", func() {
		testCredential := models.CredentialModel{
			HTTPReferrerRestriction: "*",
			Platform:                "web",
		}
		credentialRepo.CreateOne(testCredential)
		req, err := http.NewRequest("GET", "/?limit=50&offset=0", nil)
		if err != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", err))
		}

		w := httptest.NewRecorder()
		credentialController.ListCredentials(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

	It("Get One Credential", func() {
		testCredential := models.CredentialModel{
			HTTPReferrerRestriction: "*",
			Platform:                "web",
		}
		_, createOneErr := credentialRepo.CreateOne(testCredential)
		if createOneErr != nil {
			utils.Error(createOneErr.Message)
		}
		Expect(createOneErr).To(BeNil())

		path := fmt.Sprintf("/credentials/%v", testCredential.ID)
		req, err := http.NewRequest("GET", path, nil)
		if err != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", err))
		}

		w := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/credentials/{id}", credentialController.GetOneCredential)
		router.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

	It("Update One Credential", func() {
		testCredential := models.CredentialModel{
			HTTPReferrerRestriction: "*",
			Platform:                "web",
		}
		_, err := credentialRepo.CreateOne(testCredential)
		if err != nil {
			utils.Error(err.Message)
		}
		Expect(err).To(BeNil())

		newHTTPReferrerRestriction := "http://scheduler0.com"
		updateBody := models.CredentialModel{
			HTTPReferrerRestriction: newHTTPReferrerRestriction,
			Platform:                "web",
		}
		jsonTestCredentialBody, toJSONErr := updateBody.ToJSON()
		if toJSONErr != nil {
			utils.Error("\t\tcannot create http request %v", toJSONErr)
		}

		path := fmt.Sprintf("/credentials/%v", testCredential.ID)
		req, httpRequestErr := http.NewRequest("PUT", path, strings.NewReader(string(jsonTestCredentialBody)))
		if httpRequestErr != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", httpRequestErr))
		}

		w := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/credentials/{id}", credentialController.UpdateOneCredential)
		router.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

	It("Delete One Credential", func() {
		testCredential := models.CredentialModel{
			HTTPReferrerRestriction: "*",
			Platform:                "web",
		}
		_, err := credentialRepo.CreateOne(testCredential)
		if err != nil {
			utils.Error(err.Message)
		}
		Expect(err).To(BeNil())

		test2Credential := models.CredentialModel{
			HTTPReferrerRestriction: "*",
			Platform:                "web",
		}
		_, err = credentialRepo.CreateOne(test2Credential)
		if err != nil {
			utils.Error(err.Message)
		}
		Expect(err).To(BeNil())

		path := fmt.Sprintf("/credentials/%v", testCredential.ID)
		req, httpRequestErr := http.NewRequest("DELETE", path, nil)
		if httpRequestErr != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", httpRequestErr))
		}

		w := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/credentials/{id}", credentialController.DeleteOneCredential)
		router.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusNoContent))
	})

})

func TestCredential_Controller(t *testing.T) {
	utils.SetTestScheduler0Configurations()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credential Controller Suite")
}
