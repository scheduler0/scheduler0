package ios_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	"net/http"
	"scheduler0/db"
	"scheduler0/http_server/middlewares/auth"
	"scheduler0/http_server/middlewares/auth/ios"
	"scheduler0/repository"
	"scheduler0/repository/fixtures"
	store2 "scheduler0/server/cluster"
	"scheduler0/service"
	"scheduler0/utils"
	"testing"
)

var _ = Describe("IOS Auth Test", func() {

	dbConnection := db.GetTestDBConnection()
	store := store2.NewStore(dbConnection, nil)
	credentialRepo := repository.NewCredentialRepo(&store)
	ctx := context.Background()
	credentialService := service.NewCredentialService(credentialRepo, ctx)

	BeforeEach(func() {
		db.TeardownTestDB()
		db.PrepareTestDB()
	})

	It("Should identify request from IOS apps", func() {
		req, err := http.NewRequest("POST", "/", nil)
		Expect(err).To(BeNil())

		credentialFixture := fixtures.CredentialFixture{}
		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
		credentialTransformer := credentialTransformers[0]

		credentialTransformer.Platform = repository.IOSPlatform

		_, createError := credentialService.CreateNewCredential(credentialTransformer)
		if createError != nil {
			utils.Error(fmt.Sprintf("Error: %v", createError.Message))
		}

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		req.Header.Set(auth.IOSBundleHeader, credentialTransformer.IOSBundleIDRestriction)

		Expect(ios.IsIOSClient(req)).To(BeTrue())
	})

	It("Should not identify request from non IOS apps", func() {
		req, err := http.NewRequest("POST", "/", nil)
		Expect(err).To(BeNil())

		credentialFixture := fixtures.CredentialFixture{}
		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)

		_, createError := credentialService.CreateNewCredential(credentialTransformers[0])
		if createError != nil {
			utils.Warn(fmt.Sprintf("Error: %v", createError.Message))
		}

		credentialTransformer := credentialTransformers[0]

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		req.Header.Set(auth.AndroidPackageIDHeader, credentialTransformer.AndroidPackageNameRestriction)
		Expect(ios.IsIOSClient(req)).ToNot(BeTrue())

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		req.Header.Set(auth.SecretKeyHeader, credentialTransformer.ApiSecret)
		Expect(ios.IsIOSClient(req)).ToNot(BeTrue())

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		Expect(ios.IsIOSClient(req)).ToNot(BeTrue())
	})

	It("Should identify authorized request from ios apps", func() {
		req, err := http.NewRequest("POST", "/", nil)
		Expect(err).To(BeNil())

		credentialFixture := fixtures.CredentialFixture{}
		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
		credentialTransformer := credentialTransformers[0]

		credentialTransformer.Platform = repository.IOSPlatform

		credentialManagerUUID, createError := credentialService.CreateNewCredential(credentialTransformer)
		if createError != nil {
			utils.Error(fmt.Sprintf("Error: %v", createError.Message))
		}

		updatedCredentialTransformer, getError := credentialService.FindOneCredentialByID(credentialManagerUUID)
		if getError != nil {
			utils.Error(fmt.Sprintf("Error: %v", getError.Error()))
		}

		if updatedCredentialTransformer == nil {
			Expect(updatedCredentialTransformer).ToNot(BeNil())
			return
		}

		req.Header.Set(auth.APIKeyHeader, updatedCredentialTransformer.ApiKey)
		req.Header.Set(auth.IOSBundleHeader, credentialTransformer.IOSBundleIDRestriction)

		Expect(ios.IsAuthorizedIOSClient(req, credentialService)).To(BeTrue())
	})
})

func TestIOSAuth_Middleware(t *testing.T) {
	utils.SetTestScheduler0Configurations()
	RegisterFailHandler(Fail)
	RunSpecs(t, "IOS Auth Test")
}
