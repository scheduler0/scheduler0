package managers

import (
	"cron-server/server/src/managers"
	"cron-server/server/tests"
	"testing"
)

var credentialManager = managers.CredentialManager{}

func Test_CredentialManager(t *testing.T) {
	pool := tests.GetTestPool()

	t.Log("CredentialManager.CreateOne")
	{

		t.Logf("\t\tDon't create a credential without HTTPReferrerRestriction")
		{
			_, err := credentialManager.CreateOne(pool)
			if err == nil {
				t.Fatalf("Created a new credential without HTTPReferrerRestriction")
			}
		}

		t.Logf("\t\tCreate a new credential")
		{
			credentialManager.HTTPReferrerRestriction = "*"
			_, err := credentialManager.CreateOne(pool)
			if err != nil {
				t.Fatalf("Failed to create a new crendential")
			}
		}
	}

	t.Log("CredentialManager.UpdateOne")
	{

		var oldApiKey = credentialManager.ApiKey

		t.Logf("\t\tCannot update credential api key")
		{
			credentialManager.ApiKey = "13455"

			_, err := credentialManager.UpdateOne(pool)
			if err == nil {
				t.Fatalf("Cannot update credential key")
			}
		}

		t.Logf("\t\tUpdate credential HTTPReferrerRestriction")
		{
			credentialManager.ApiKey = oldApiKey
			credentialManager.HTTPReferrerRestriction = "http://google.com"
			_, err := credentialManager.CreateOne(pool)
			if err != nil {
				t.Fatalf("Failed to update crendential")
			}
		}
	}

	t.Log("CredentialManager.DeleteOne")
	{
		t.Logf("\t\tPrevent deleting all credential")
		{
			_, err := credentialManager.DeleteOne(pool)
			if err != nil {
				t.Fatalf("Cannot delete all credentials %v", err.Error())
			}
		}
	}

}
