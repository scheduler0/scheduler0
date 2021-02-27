package auth_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"scheduler0/server/db"
	"scheduler0/server/http_server/middlewares/auth"
	"scheduler0/server/managers/credential"
	"scheduler0/server/managers/credential/fixtures"
	"scheduler0/server/service"
	"scheduler0/utils"
	"testing"
)

var _ = Describe("Android Auth Test", func() {

	db.Teardown()
	db.Prepare()

	It("Should identify request from android apps", func() {
		req, err := http.NewRequest("POST", "/", nil)
		Expect(err).To(BeNil())

		pool := db.GetTestPool()

		credentialService := service.Credential{
			Pool: pool,
		}

		credentialFixture := fixtures.CredentialFixture{}
		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
		credentialTransformer := credentialTransformers[0]

		credentialTransformer.Platform = credential.AndroidPlatform

		_, createError := credentialService.CreateNewCredential(credentialTransformer)
		if createError != nil {
			utils.Error(fmt.Sprintf("Error: %v", createError.Message))
		}

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		req.Header.Set(auth.AndroidPackageIDHeader, credentialTransformer.AndroidPackageNameRestriction)

		Expect(auth.IsAndroidClient(req)).To(BeTrue())
	})

	It("Should not identify request from non android apps", func() {
		req, err := http.NewRequest("POST", "/", nil)
		Expect(err).To(BeNil())

		pool := db.GetTestPool()

		credentialService := service.Credential{
			Pool: pool,
		}

		credentialFixture := fixtures.CredentialFixture{}
		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)

		_, createError := credentialService.CreateNewCredential(credentialTransformers[0])
		if createError != nil {
			utils.Warn(fmt.Sprintf("Error: %v", createError.Message))
		}

		credentialTransformer := credentialTransformers[0]

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		req.Header.Set(auth.IOSBundleHeader, credentialTransformer.IOSBundleIDRestriction)
		Expect(auth.IsAndroidClient(req)).ToNot(BeTrue())

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		req.Header.Set(auth.SecretKeyHeader, credentialTransformer.ApiSecret)
		Expect(auth.IsAndroidClient(req)).ToNot(BeTrue())

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		Expect(auth.IsAndroidClient(req)).ToNot(BeTrue())
	})


	It("Should identify authorized request from android apps", func() {
		req, err := http.NewRequest("POST", "/", nil)
		Expect(err).To(BeNil())

		pool := db.GetTestPool()

		credentialService := service.Credential{
			Pool: pool,
		}

		credentialFixture := fixtures.CredentialFixture{}
		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
		credentialTransformer := credentialTransformers[0]

		credentialTransformer.Platform = credential.AndroidPlatform

		credentialManagerUUID, createError := credentialService.CreateNewCredential(credentialTransformer)
		if createError != nil {
			utils.Error(fmt.Sprintf("Error: %v", createError.Message))
		}

		updatedCredentialTransformer, getError := credentialService.FindOneCredentialByUUID(credentialManagerUUID)
		if getError != nil {
			utils.Error(fmt.Sprintf("Error: %v", getError.Error()))
		}

		if updatedCredentialTransformer == nil {
			Expect(updatedCredentialTransformer).ToNot(BeNil())
			return
		}

		req.Header.Set(auth.APIKeyHeader, updatedCredentialTransformer.ApiKey)
		req.Header.Set(auth.AndroidPackageIDHeader, credentialTransformer.AndroidPackageNameRestriction)

		Expect(auth.IsAuthorizedAndroidClient(req, pool)).To(BeTrue())
	})
})


func TestAndroidAuth_Middleware(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Android Auth Test")
}