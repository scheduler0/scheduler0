package android_test

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"scheduler0/db"
	"scheduler0/http_server/middlewares/auth"
	"scheduler0/http_server/middlewares/auth/android"
	"scheduler0/repository"
	"scheduler0/repository/fixtures"
	store2 "scheduler0/server/cluster"
	"scheduler0/service"
	"scheduler0/utils"
	"testing"
)

var _ = Describe("Android Auth Test", func() {

	dbConnection := db.GetTestDBConnection()
	store := store2.NewStore(dbConnection, nil)
	credentialRepo := repository.NewCredentialRepo(&store)
	ctx := context.Background()
	credentialService := service.NewCredentialService(credentialRepo, ctx)

	BeforeEach(func() {
		db.TeardownTestDB()
		db.PrepareTestDB()
	})

	It("Should identify request from android apps", func() {
		req, err := http.NewRequest("POST", "/", nil)
		Expect(err).To(BeNil())

		credentialFixture := fixtures.CredentialFixture{}
		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
		credentialTransformer := credentialTransformers[0]

		credentialTransformer.Platform = repository.AndroidPlatform

		_, createError := credentialService.CreateNewCredential(credentialTransformer)
		if createError != nil {
			utils.Error(fmt.Sprintf("Error: %v", createError.Message))
		}

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		req.Header.Set(auth.AndroidPackageIDHeader, credentialTransformer.AndroidPackageNameRestriction)

		Expect(android.IsAndroidClient(req)).To(BeTrue())
	})

	It("Should not identify request from non android apps", func() {
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
		req.Header.Set(auth.IOSBundleHeader, credentialTransformer.IOSBundleIDRestriction)
		Expect(android.IsAndroidClient(req)).ToNot(BeTrue())

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		req.Header.Set(auth.SecretKeyHeader, credentialTransformer.ApiSecret)
		Expect(android.IsAndroidClient(req)).ToNot(BeTrue())

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)
		Expect(android.IsAndroidClient(req)).ToNot(BeTrue())
	})

	It("Should identify authorized request from android apps", func() {
		req, err := http.NewRequest("POST", "/", nil)
		Expect(err).To(BeNil())

		credentialFixture := fixtures.CredentialFixture{}
		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
		credentialTransformer := credentialTransformers[0]

		credentialTransformer.Platform = repository.AndroidPlatform

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
		req.Header.Set(auth.AndroidPackageIDHeader, credentialTransformer.AndroidPackageNameRestriction)

		Expect(android.IsAuthorizedAndroidClient(req, credentialService)).To(BeTrue())
	})
})

func TestAndroidAuth_Middleware(t *testing.T) {
	utils.SetTestScheduler0Configurations()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Android Auth Test")
}
