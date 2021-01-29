package managers

import (
	"github.com/magiconair/properties/assert"
	"github.com/victorlenerd/scheduler0/server/src/managers"
	"github.com/victorlenerd/scheduler0/server/tests"
	"testing"
)


func Test_CredentialManager(t *testing.T) {
	pool := tests.GetTestPool()
	credentialManager := managers.CredentialManager{}

	t.Log("CredentialManager.CreateOne")
	{

		t.Logf("\t\tDon't create a credential without HTTPReferrerRestriction")
		{
			_, err := credentialManager.CreateOne(pool)
			if err == nil {
				t.Fatalf("\t\t [ERROR] Created a new credential without HTTPReferrerRestriction")
			}
		}

		t.Logf("\t\tCreate a new credential")
		{
			credentialManager.HTTPReferrerRestriction = "*"
			_, err := credentialManager.CreateOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Failed to create a new crendential" + err.Error())
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
				t.Fatalf("\t\t [ERROR] Cannot update credential key")
			}
		}

		t.Logf("\t\tUpdate credential HTTPReferrerRestriction")
		{
			credentialManager.ApiKey = oldApiKey
			credentialManager.HTTPReferrerRestriction = "http://google.com"
			_, err := credentialManager.CreateOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] Failed to update crendential")
			}
		}
	}

	t.Log("CredentialManager.DeleteOne")
	{
		t.Logf("\t\tPrevent deleting all credential")
		{
			_, err := credentialManager.DeleteOne(pool)
			if err != nil {
				t.Fatalf("\t\t [ERROR] \t\t [ERROR] Cannot delete all credentials %v", err.Error())
			}
		}
	}

	t.Log("CredentialManager.GetAll")
	{

		credentialManager.HTTPReferrerRestriction = "*"

		for i := 0; i < 1000; i++ {
			_, err := credentialManager.CreateOne(pool)


			if err != nil {
				t.Fatalf("\t\t [ERROR] Failed to create a new crendential" + err.Error())
			}
		}

		credentials, err := credentialManager.GetAll(pool, 0, 100, "date_created")
		if err != nil {
			t.Fatalf("\t\t [ERROR] Failed to fetch crendentials" + err.Error())
		}

		assert.Equal(t, 100, len(credentials))
	}
}
