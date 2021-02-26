package credential_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"scheduler0/server/db"
	"scheduler0/server/managers/credential"
	"scheduler0/utils"
	"testing"
)

var _ = Describe("Credential Manager", func() {
	pool := db.GetTestPool()

	db.Teardown()
	db.Prepare()

	It("Don't create a credential without a platform", func() {
		credentialManager := credential.Manager{}
		_, err := credentialManager.CreateOne(pool)
		if err == nil {
			utils.Error("[ERROR] Created a new credential without platform")
		}
		Expect(err).ToNot(BeNil())
	})

	Context("Web platform credential", func() {
		It("Should not create web platform credential without HTTPReferrerRestriction or IPRestriction", func() {
			credentialManager := credential.Manager{}
			credentialManager.Platform = "web"
			_, err := credentialManager.CreateOne(pool)
			if err == nil {
				utils.Error("[ERROR] Created a new credential without HTTPReferrerRestriction")
			}
			Expect(err).ToNot(BeNil())
		})

		It("Should create web platform credential with HTTPReferrerRestriction", func() {
			credentialManager := credential.Manager{}
			credentialManager.Platform = "web"
			credentialManager.HTTPReferrerRestriction = "*"
			_, err := credentialManager.CreateOne(pool)
			if err != nil {
				utils.Error(fmt.Sprintf("[ERROR] failed to create web credential with http restriction: - %v", err.Message))
			}
			Expect(err).To(BeNil())
		})

		It("Should create web platform credential with IPRestriction", func() {
			credentialManager := credential.Manager{}
			credentialManager.Platform = "web"
			credentialManager.IPRestriction = "*"
			_, err := credentialManager.CreateOne(pool)
			if err != nil {
				utils.Error(fmt.Sprintf("[ERROR] failed to create web credential with ip restriction %v", err.Message))
			}
			Expect(err).To(BeNil())
		})
	})

	Context("Android platform credential", func() {
		It("Should not create android platform credential without android package name restriction", func() {
			credentialManager := credential.Manager{}
			credentialManager.Platform  = "android"
			_, err := credentialManager.CreateOne(pool)
			if err == nil {
				utils.Error("[ERROR] Created an android credential without package name restriction")
			}
			Expect(err).ToNot(BeNil())
		})

		It("Should create android platform credential with android package name restriction", func() {
			credentialManager := credential.Manager{}
			credentialManager.Platform  = "android"
			credentialManager.AndroidPackageNameRestriction = "com.android.org"
			_, err := credentialManager.CreateOne(pool)
			if err != nil {
				utils.Error(fmt.Sprintf("[ERROR] failed to create an android credential with package name restriction %v", err.Message))
			}
			Expect(err).To(BeNil())
		})
	})

	Context("iOS platform credential", func() {
		It("Should not create ios platform credential without bundle id restriction", func() {
			credentialManager := credential.Manager{}
			credentialManager.Platform = "ios"
			_, err := credentialManager.CreateOne(pool)
			if err == nil {
				utils.Error("[ERROR] Created an ios credential bundle id restriction")
			}
			Expect(err).ToNot(BeNil())
		})

		It("Should create ios platform credential with android package name restriction", func() {
			credentialManager := credential.Manager{}
			credentialManager.Platform  = "ios"
			credentialManager.IOSBundleIDRestriction = "com.ios.org"
			_, err := credentialManager.CreateOne(pool)
			if err != nil {
				utils.Error(fmt.Sprintf("[ERROR] failed to create an ios credential with package name restriction %v", err.Message))
			}
			Expect(err).To(BeNil())
		})
	})

	Context("Server platform credential", func() {
		It("Should create server credential", func() {
			credentialManager := credential.Manager{}
			credentialManager.Platform = "server"
			_, err := credentialManager.CreateOne(pool)
			if err != nil {
				utils.Error("[ERROR] Failed to create a server credential" + err.Message)
			}

			Expect(err).To(BeNil())
			Expect(len(credentialManager.ApiSecret) > 60).To(BeTrue())
			Expect(len(credentialManager.ApiKey) > 60).To(BeTrue())
		})
	})

	It("Should not update credential api key and secret", func() {
		credentialManager := credential.Manager{}
		credentialManager.Platform = "server"
		_, err := credentialManager.CreateOne(pool)
		if err != nil {
			utils.Error("[ERROR] Failed to create a new credential" + err.Message)
		}

		Expect(err).To(BeNil())

		credentialManager.ApiKey = "13455"
		_, updateErr := credentialManager.UpdateOne(pool)
		if updateErr == nil {
			utils.Error("[ERROR] should not update credential key")
		}
		Expect(updateErr).ToNot(BeNil())
	})

	It("Update credential HTTPReferrerRestriction", func() {
		credentialManager := credential.Manager{
			Platform: "web",
			HTTPReferrerRestriction: "*",
		}
		_, err := credentialManager.CreateOne(pool)
		if err != nil {
			utils.Error("[ERROR] Failed to create a new credential" + err.Message)
		}

		Expect(err).To(BeNil())

		credentialManager.HTTPReferrerRestriction = "http://google.com"
		_, err = credentialManager.CreateOne(pool)
		if err != nil {
			utils.Error(err.Message)
		}
		Expect(err).ToNot(BeNil())
	})

	It("Delete one credential", func() {
		credentialManager := credential.Manager{
			Platform: "server",
		}
		_, err := credentialManager.CreateOne(pool)
		if err != nil {
			utils.Error("[ERROR] Failed to create a new credential" + err.Message)
		}
		Expect(err).To(BeNil())
		rowAffected, deleteErr := credentialManager.DeleteOne(pool)
		Expect(err).To(BeNil())
		if err != nil {
			utils.Error(deleteErr.Error())
		}

		Expect(rowAffected == 1).To(BeTrue())
	})

	It("CredentialManager.List", func() {
		credentialManager := credential.Manager{}
		credentials, err := credentialManager.GetAll(pool, 0, 100, "date_created")
		Expect(err).To(BeNil())
		if err != nil {
			utils.Error(fmt.Sprintf("CredentialManager.List::Error::%v", err.Message))
		}
		Expect(len(credentials) > 0).To(BeTrue())
	})

	It("Prevent deleting all credential", func() {
		credentialManager := credential.Manager{}
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
