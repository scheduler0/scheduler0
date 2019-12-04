package models

import (
	"context"
	"cron-server/server/repository"
	"testing"
)

var (
	credentialModel = Credential{}
	credentialCtx   = context.Background()
)

func TestCredential_CreateOne(t *testing.T) {
	var pool, _ = repository.NewPool(repository.CreateConnection, 1)
	defer pool.Close()

	t.Log("Don't create a credential without HTTPReferrerRestriction")
	{
		_, err := credentialModel.CreateOne(pool, credentialCtx)
		if err == nil {
			t.Fatalf("Created a new credential without HTTPReferrerRestriction")
		}
	}

	t.Log("Create a new credential")
	{
		credentialModel.HTTPReferrerRestriction = "*"
		_, err := credentialModel.CreateOne(pool, credentialCtx)
		if err != nil {
			t.Fatalf("Failed to create a new crendential")
		}
	}
}

func TestCredential_UpdateOne(t *testing.T) {
	var pool, _ = repository.NewPool(repository.CreateConnection, 1)
	defer pool.Close()

	var oldApiKey = credentialModel.ApiKey

	t.Log("Cannot update credential api key")
	{
		credentialModel.ApiKey = "13455"

		_, err := credentialModel.UpdateOne(pool, credentialCtx)
		if err == nil {
			t.Fatalf("Cannot update credential key")
		}
	}

	t.Log("Update credential HTTPReferrerRestriction")
	{
		credentialModel.ApiKey = oldApiKey
		credentialModel.HTTPReferrerRestriction = "http://google.com"
		_, err := credentialModel.CreateOne(pool, credentialCtx)
		if err != nil {
			t.Fatalf("Failed to update crendential")
		}
	}
}

func TestCredential_DeleteOne(t *testing.T) {
	var pool, _ = repository.NewPool(repository.CreateConnection, 1)
	defer pool.Close()

	t.Log("Prevent deleting all credential")
	{
		_, err := credentialModel.DeleteOne(pool, credentialCtx)
		if err != nil {
			t.Fatalf("Cannot delete all credentials %v", err.Error())
		}
	}
}
