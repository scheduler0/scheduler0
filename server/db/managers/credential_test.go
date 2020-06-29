package managers

import (
	"cron-server/server"
	"cron-server/server/db"
	"io"
	"testing"
)

var (
	credentialDomain = CredentialManager{}
)

func TestCredential_CreateOne(t *testing.T) {
	pool, err := db.NewPool(func() (closer io.Closer, err error) {
		return db.CreateConnectionEnv("TEST")
	}, 1)

	if err


	t.Log("Don't create a credential without HTTPReferrerRestriction")
	{
		_, err := credentialDomain.CreateOne(pool)
		if err == nil {
			t.Fatalf("Created a new credential without HTTPReferrerRestriction")
		}
	}

	t.Log("Create a new credential")
	{
		credentialDomain.HTTPReferrerRestriction = "*"
		_, err := credentialDomain.CreateOne(pool)
		if err != nil {
			t.Fatalf("Failed to create a new crendential")
		}
	}
}

func TestCredential_UpdateOne(t *testing.T) {
	var pool, _ = main.GetTestDBPool()
	defer pool.Close()

	var oldApiKey = credentialDomain.ApiKey

	t.Log("Cannot update credential api key")
	{
		credentialDomain.ApiKey = "13455"

		_, err := credentialDomain.UpdateOne(pool)
		if err == nil {
			t.Fatalf("Cannot update credential key")
		}
	}

	t.Log("Update credential HTTPReferrerRestriction")
	{
		credentialDomain.ApiKey = oldApiKey
		credentialDomain.HTTPReferrerRestriction = "http://google.com"
		_, err := credentialDomain.CreateOne(pool)
		if err != nil {
			t.Fatalf("Failed to update crendential")
		}
	}
}

func TestCredential_DeleteOne(t *testing.T) {
	var pool, _ = main.GetTestDBPool()
	defer pool.Close()

	t.Log("Prevent deleting all credential")
	{
		_, err := credentialDomain.DeleteOne(pool)
		if err != nil {
			t.Fatalf("Cannot delete all credentials %v", err.Error())
		}
	}

	main.TruncateDBAfterTest()
}
