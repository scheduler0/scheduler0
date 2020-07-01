package managers

import (
	"cron-server/server/src/managers"
	"cron-server/server/tests"
	"testing"
)

var credentialManager = managers.CredentialManager{}

func TestCredential_CreateOne(t *testing.T) {
	pool := tests.GetTestPool()

	t.Log("Don't create a credential without HTTPReferrerRestriction")
	{
		_, err := credentialManager.CreateOne(pool)
		if err == nil {
			t.Fatalf("Created a new credential without HTTPReferrerRestriction")
		}
	}

	t.Log("Create a new credential")
	{
		credentialManager.HTTPReferrerRestriction = "*"
		_, err := credentialManager.CreateOne(pool)
		if err != nil {
			t.Fatalf("Failed to create a new crendential")
		}
	}
}

func TestCredential_UpdateOne(t *testing.T) {
	pool := tests.GetTestPool()

	var oldApiKey = credentialManager.ApiKey

	t.Log("Cannot update credential api key")
	{
		credentialManager.ApiKey = "13455"

		_, err := credentialManager.UpdateOne(pool)
		if err == nil {
			t.Fatalf("Cannot update credential key")
		}
	}

	t.Log("Update credential HTTPReferrerRestriction")
	{
		credentialManager.ApiKey = oldApiKey
		credentialManager.HTTPReferrerRestriction = "http://google.com"
		_, err := credentialManager.CreateOne(pool)
		if err != nil {
			t.Fatalf("Failed to update crendential")
		}
	}
}

func TestCredential_DeleteOne(t *testing.T) {
	pool := tests.GetTestPool()

	t.Log("Prevent deleting all credential")
	{
		_, err := credentialManager.DeleteOne(pool)
		if err != nil {
			t.Fatalf("Cannot delete all credentials %v", err.Error())
		}
	}
}
