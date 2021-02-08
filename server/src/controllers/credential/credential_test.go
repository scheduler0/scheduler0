package credential_test

import (
	"fmt"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/victorlenerd/scheduler0/server/src/controllers/credential"
	"github.com/victorlenerd/scheduler0/server/src/transformers"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"github.com/victorlenerd/scheduler0/server/tests"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)


var _ = Describe("Credential Controller", func () {

	BeforeEach(func() {
		tests.Teardown()
		tests.Prepare()
	})

	pool := tests.GetTestPool()
	credentialController := credential.CredentialController{ Pool: pool}

	It("Creating A New Credential", func() {
		testCredential := transformers.Credential{HTTPReferrerRestriction: "*"}
		jsonTestCredentialBody, err := testCredential.ToJson()
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

	It("Get All Credentials", func () {
		req, err := http.NewRequest("GET", "/?limit=50&offset=0", nil)
		if err != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", err))
		}

		w := httptest.NewRecorder()
		credentialController.List(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

	It("Get One Credential", func () {
		testCredential := transformers.Credential{HTTPReferrerRestriction: "*"}
		credentialManager := testCredential.ToManager()
		_, err := credentialManager.CreateOne(pool)
		if err != nil {
			utils.Error(err.Error())
		}
		Expect(err).To(BeNil())

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
		testCredential := transformers.Credential{HTTPReferrerRestriction: "*"}
		credentialManager := testCredential.ToManager()
		_, err := credentialManager.CreateOne(pool)
		if err != nil {
			utils.Error(err.Error())
		}
		Expect(err).To(BeNil())

		newHTTPReferrerRestriction :=  "http://scheduler0.com"
		updateBody := transformers.Credential{HTTPReferrerRestriction: newHTTPReferrerRestriction}
		jsonTestCredentialBody, err := updateBody.ToJson()
		if err != nil {
			utils.Error("\t\tcannot create http request %v", err)
		}

		path := fmt.Sprintf("/credentials/%s", credentialManager.UUID)
		req, err := http.NewRequest("PUT", path, strings.NewReader(string(jsonTestCredentialBody)))
		if err != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", err))
		}

		w := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/credentials/{uuid}", credentialController.UpdateOne)
		router.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
	})

	It("Delete One Credential", func () {
		testCredential := transformers.Credential{HTTPReferrerRestriction: "*"}
		credentialManager := testCredential.ToManager()
		_, err := credentialManager.CreateOne(pool)
		if err != nil {
			utils.Error(err.Error())
		}
		Expect(err).To(BeNil())

		path := fmt.Sprintf("/credentials/%s", credentialManager.UUID)
		req, err := http.NewRequest("DELETE", path, nil)
		if err != nil {
			utils.Error(fmt.Sprintf("cannot create http request %v", err))
		}

		w := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/credentials/{uuid}", credentialController.DeleteOne)
		router.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusNoContent))
	})

})

func TestCredential_Controller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credential Controller Suite")
}