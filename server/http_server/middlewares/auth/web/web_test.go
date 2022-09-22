package web_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	"net/http"
	store2 "scheduler0/server/cluster"
	"scheduler0/server/db"
	"scheduler0/server/http_server/middlewares/auth"
	"scheduler0/server/http_server/middlewares/auth/web"
	"scheduler0/server/repository"
	"scheduler0/server/repository/fixtures"
	"scheduler0/server/service"
	"scheduler0/utils"
	"testing"
)

var _ = Describe("Web Auth Test", func() {

	dbConnection := db.GetTestDBConnection()
	store := store2.NewStore(dbConnection, nil)
	credentialRepo := repository.NewCredentialRepo(&store)
	ctx := context.Background()
	credentialService := service.NewCredentialService(credentialRepo, ctx)

	BeforeEach(func() {
		db.TeardownTestDB()
		db.PrepareTestDB()
	})

	It("Should identify request from web clients", func() {
		req, err := http.NewRequest("POST", "/", nil)
		Expect(err).To(BeNil())

		credentialFixture := fixtures.CredentialFixture{}
		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
		credentialTransformer := credentialTransformers[0]

		credentialTransformer.Platform = repository.WebPlatform
		credentialTransformer.HTTPReferrerRestriction = credentialFixture.HTTPReferrerRestriction

		_, createError := credentialService.CreateNewCredential(credentialTransformer)
		if createError != nil {
			utils.Error(fmt.Sprintf("Error: %v", createError.Message))
		}

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)

		Expect(web.IsWebClient(req)).To(BeTrue())
	})

})

func TestWebAuth_Middleware(t *testing.T) {
	utils.SetTestScheduler0Configurations()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Web Auth Test")
}
