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

var _ = Describe("Web Auth Test", func() {

	db.Teardown()
	db.Prepare()

	It("Should identify request from web clients", func() {
		req, err := http.NewRequest("POST", "/", nil)
		Expect(err).To(BeNil())

		pool := db.GetTestPool()

		credentialService := service.Credential{
			Pool: pool,
		}

		credentialFixture := fixtures.CredentialFixture{}
		credentialTransformers := credentialFixture.CreateNCredentialTransformer(1)
		credentialTransformer := credentialTransformers[0]

		credentialTransformer.Platform = credential.WebPlatform
		credentialTransformer.HTTPReferrerRestriction = credentialFixture.HTTPReferrerRestriction

		_, createError := credentialService.CreateNewCredential(credentialTransformer)
		if createError != nil {
			utils.Error(fmt.Sprintf("Error: %v", createError.Message))
		}

		req.Header.Set(auth.APIKeyHeader, credentialTransformer.ApiKey)

		Expect(auth.IsWebClient(req)).To(BeTrue())
	})

})


func TestWebAuth_Middleware(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Web Auth Test")
}
