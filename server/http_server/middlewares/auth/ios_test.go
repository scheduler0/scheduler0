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

var _ = Describe("IOS Auth Test", func() {

	db.Teardown()
	db.Prepare()

	It("Should identify request from IOS apps", func() {
		req, err := http.NewRequest("POST", "/", nil)
		Expect(err).To(BeNil())

		pool := db.GetTestPool()

		credentialService := service.Credential{
			Pool: pool,
		}

		credentialFixture := fixtures.CredentialFixture{}
		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
		credentialTransformer := credentialTransformers[0]

		credentialTransformer.Platform = credential.IOSPlatform

		_, createError := credentialService.CreateNewCredential(credentialTransformer)
		if createError != nil {
			utils.Error(fmt.Sprintf("Error: %v", createError.Message))
		}

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		req.Header.Set(auth.IOSBundleHeader, credentialTransformer.IOSBundleIDRestriction)

		Expect(auth.IsIOSClient(req)).To(BeTrue())
	})

	It("Should not identify request from non IOS apps", func() {
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
		req.Header.Set(auth.AndroidPackageIDHeader, credentialTransformer.AndroidPackageNameRestriction)
		Expect(auth.IsIOSClient(req)).ToNot(BeTrue())

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		req.Header.Set(auth.SecretKeyHeader, credentialTransformer.ApiSecret)
		Expect(auth.IsIOSClient(req)).ToNot(BeTrue())

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		Expect(auth.IsIOSClient(req)).ToNot(BeTrue())
	})

	It("Should identify authorized request from ios apps", func() {
		req, err := http.NewRequest("POST", "/", nil)
		Expect(err).To(BeNil())

		pool := db.GetTestPool()

		credentialService := service.Credential{
			Pool: pool,
		}

		credentialFixture := fixtures.CredentialFixture{}
		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
		credentialTransformer := credentialTransformers[0]

		credentialTransformer.Platform = credential.IOSPlatform

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
		req.Header.Set(auth.IOSBundleHeader, credentialTransformer.IOSBundleIDRestriction)

		Expect(auth.IsAuthorizedIOSClient(req, pool)).To(BeTrue())
	})
})


func TestIOSAuth_Middleware(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IOS Auth Test")
}
