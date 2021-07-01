package credential_test

import (
	"fmt"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"scheduler0/server/db"
	"scheduler0/server/http_server/controllers/credential"
	"scheduler0/server/transformers"
	"scheduler0/utils"
	"strings"
	"testing"
)

var _ = Describe("Credential Controller", func() {

	BeforeEach(func() {
		db.Teardown()
		db.Prepare()
	})

	DBConnection := db.GetTestDBConnection()
	credentialController := credential.Controller{DBConnection: DBConnection}

	It("Creating A New Credential", func() {
		testCredential := transformers.Credential{
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
		credentialController.CreateOne(w, req)

		Expect(w.Code).To(Equal(http.StatusCreated))
	})

	It("Get All Credentials", func() {
		testCredential := transformers.Credential{
			HTTPReferrerRestriction: "*",
			Platform:                "web",
		}
		manager := testCredential.ToManager()
		manager.CreateOne(DBConnection)
		req, err := http.NewRequest("GET", "/?limit=50&offset=0", nil)
		if err != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", err))
		}

		w := httptest.NewRecorder()
		credentialController.List(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

	It("Get One Credential", func() {
		testCredential := transformers.Credential{
			HTTPReferrerRestriction: "*",
			Platform:                "web",
		}
		credentialManager := testCredential.ToManager()
		_, createOneErr := credentialManager.CreateOne(DBConnection)
		if createOneErr != nil {
			utils.Error(createOneErr.Message)
		}
		Expect(createOneErr).To(BeNil())

		path := fmt.Sprintf("/credentials/%s", credentialManager.UUID)
		req, err := http.NewRequest("GET", path, nil)
		if err != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", err))
		}

		w := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/credentials/{uuid}", credentialController.GetOne)
		router.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

	It("Update One Credential", func() {
		testCredential := transformers.Credential{
			HTTPReferrerRestriction: "*",
			Platform:                "web",
		}
		credentialManager := testCredential.ToManager()
		_, err := credentialManager.CreateOne(DBConnection)
		if err != nil {
			utils.Error(err.Message)
		}
		Expect(err).To(BeNil())

		newHTTPReferrerRestriction := "http://scheduler0.com"
		updateBody := transformers.Credential{
			HTTPReferrerRestriction: newHTTPReferrerRestriction,
			Platform:                "web",
		}
		jsonTestCredentialBody, toJSONErr := updateBody.ToJSON()
		if toJSONErr != nil {
			utils.Error("\t\tcannot create http request %v", toJSONErr)
		}

		path := fmt.Sprintf("/credentials/%s", credentialManager.UUID)
		req, httpRequestErr := http.NewRequest("PUT", path, strings.NewReader(string(jsonTestCredentialBody)))
		if httpRequestErr != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", httpRequestErr))
		}

		w := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/credentials/{uuid}", credentialController.UpdateOne)
		router.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

	It("Delete One Credential", func() {
		testCredential := transformers.Credential{
			HTTPReferrerRestriction: "*",
			Platform:                "web",
		}
		credentialManager := testCredential.ToManager()
		_, err := credentialManager.CreateOne(DBConnection)
		if err != nil {
			utils.Error(err.Message)
		}
		Expect(err).To(BeNil())

		test2Credential := transformers.Credential{
			HTTPReferrerRestriction: "*",
			Platform:                "web",
		}
		credential2Manager := test2Credential.ToManager()
		_, err = credential2Manager.CreateOne(DBConnection)
		if err != nil {
			utils.Error(err.Message)
		}
		Expect(err).To(BeNil())

		path := fmt.Sprintf("/credentials/%s", credentialManager.UUID)
		req, httpRequestErr := http.NewRequest("DELETE", path, nil)
		if httpRequestErr != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", httpRequestErr))
		}

		w := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/credentials/{uuid}", credentialController.DeleteOne)
		router.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusNoContent))
	})

})

func TestCredential_Controller(t *testing.T) {
	utils.SetTestScheduler0Configurations()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credential Controller Suite")
}
