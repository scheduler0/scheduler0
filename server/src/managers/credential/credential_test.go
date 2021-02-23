package credential_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"scheduler0/server/src/managers/credential"
	"scheduler0/server/src/utils"
	"scheduler0/server/tests"
	"testing"
)

var _ = Describe("Credential Manager", func() {
	pool := tests.GetTestPool()

	tests.Teardown()
	tests.Prepare()

	It("Don't create a credential without HTTPReferrerRestriction", func() {
		credentialManager := credential.CredentialManager{}
		_, err := credentialManager.CreateOne(pool)
		if err == nil {
			utils.Error("[ERROR] Created a new credential without HTTPReferrerRestriction")
		}
		Expect(err).ToNot(BeNil())
	})

	It("Create a new credential", func() {
		credentialManager := credential.CredentialManager{}
		credentialManager.HTTPReferrerRestriction = "*"
		_, err := credentialManager.CreateOne(pool)
		if err != nil {
			utils.Error("[ERROR] Failed to create a new credential" + err.Error())
		}
		Expect(err).To(BeNil())
	})

	It("Cannot update credential api key", func() {
		credentialManager := credential.CredentialManager{}
		credentialManager.HTTPReferrerRestriction = "*"
		_, err := credentialManager.CreateOne(pool)
		if err != nil {
			utils.Error("[ERROR] Failed to create a new credential" + err.Error())
		}

		Expect(err).To(BeNil())

		credentialManager.ApiKey = "13455"
		_, err = credentialManager.UpdateOne(pool)
		if err == nil {
			utils.Error("[ERROR] Cannot update credential key")
		}
		Expect(err).ToNot(BeNil())
	})

	It("Update credential HTTPReferrerRestriction", func() {
		credentialManager := credential.CredentialManager{}
		credentialManager.HTTPReferrerRestriction = "*"
		_, err := credentialManager.CreateOne(pool)
		if err != nil {
			utils.Error("[ERROR] Failed to create a new credential" + err.Error())
		}

		Expect(err).To(BeNil())

		credentialManager.HTTPReferrerRestriction = "http://google.com"
		_, err = credentialManager.CreateOne(pool)
		if err != nil {
			utils.Error(err.Error())
		}
		Expect(err).ToNot(BeNil())
	})

	It("Delete one credential", func() {
		credentialManager := credential.CredentialManager{}
		credentialManager.HTTPReferrerRestriction = "*"
		_, err := credentialManager.CreateOne(pool)
		if err != nil {
			utils.Error("[ERROR] Failed to create a new credential" + err.Error())
		}
		Expect(err).To(BeNil())
		rowAffected, err := credentialManager.DeleteOne(pool)
		Expect(err).To(BeNil())
		if err != nil {
			utils.Error(err.Error())
		}

		Expect(rowAffected == 1).To(BeTrue())
	})

	It("CredentialManager.List", func() {
		credentialManager := credential.CredentialManager{}
		credentials, err := credentialManager.GetAll(pool, 0, 100, "date_created")
		Expect(err).To(BeNil())
		if err != nil {
			utils.Error(fmt.Sprintf("CredentialManager.List::Error::%v", err.Message))
		}
		Expect(len(credentials) > 0).To(BeTrue())
	})

	It("Prevent deleting all credential", func() {
		credentialManager := credential.CredentialManager{}
		credentials, err := credentialManager.GetAll(pool, 0, 100, "date_created")
		Expect(err).To(BeNil())
		if err != nil {
			utils.Error(err.Message)
		}

		for i := 0; i < len(credentials)-1; i++ {
			_, err := credentials[i].DeleteOne(pool)
			Expect(err).To(BeNil())
		}

		_, deleteCredentialError := credentials[len(credentials)-1].DeleteOne(pool)
		Expect(deleteCredentialError).ToNot(BeNil())
	})

})

func TestCredential_Manager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credential Manager Suite")
}
