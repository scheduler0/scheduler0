package fixtures

import (
	"fmt"
	"github.com/bxcodec/faker"
	"scheduler0/server/transformers"
	"scheduler0/utils"
)

// CredentialFixture credential fixture
type CredentialFixture struct {
	Platform                      string `faker:"word"`
	UUID                          string `faker:"uuid_hyphenated"`
	ApiKey                        string `faker:"ipv6"`
	ApiSecret                     string `faker:"ipv6"`
	HTTPReferrerRestriction       string `faker:"domain_name"`
	IPRestrictionRestriction      string `faker:"ipv4"`
	AndroidPackageNameRestriction string `faker:"domain_name"`
	IOSBundleIDRestriction        string `faker:"domain_name"`
}

// CreateNCredentialTransformer creates N number of credential transformers
func (credentialFixture *CredentialFixture) CreateNCredentialTransformer(n int) []transformers.Credential {
	credentialTransformers := []transformers.Credential{}

	for i := 0; i < n; i++ {
		err := faker.FakeData(credentialFixture)
		if err != nil {
			utils.Error(fmt.Sprintf("error creating fixture %v", err.Error()))
		}

		credentialTransformer := transformers.Credential{
			Platform:                      credentialFixture.Platform,
			ApiKey:                        credentialFixture.ApiKey,
			ApiSecret:                     credentialFixture.ApiSecret,
			IPRestrictionRestriction:      credentialFixture.IPRestrictionRestriction,
			HTTPReferrerRestriction:       credentialFixture.HTTPReferrerRestriction,
			AndroidPackageNameRestriction: credentialFixture.AndroidPackageNameRestriction,
			IOSBundleIDRestriction:        credentialFixture.IOSBundleIDRestriction,
		}

		credentialTransformers = append(credentialTransformers, credentialTransformer)
	}

	return credentialTransformers
}
