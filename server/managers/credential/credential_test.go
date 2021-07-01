package credential_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"scheduler0/server/db"
	"scheduler0/server/managers/credential"
	"scheduler0/utils"
	"testing"
)

func Test_CredentialManager(t *testing.T) {
	utils.SetTestScheduler0Configurations()
	dbConnection := db.GetTestDBConnection()
	forceDelete := func(credentialManager *credential.Manager) {
		_, deleteErr := dbConnection.Model(credentialManager).Where("uuid = ?", credentialManager.UUID).Delete()
		utils.CheckErr(deleteErr)
	}

	t.Run("Should not create a credential without a platform", func(t *testing.T) {
		credentialManager := credential.Manager{}
		_, err := credentialManager.CreateOne(dbConnection)
		if err == nil {
			utils.Error("[ERROR] Created a new credential without platform")
		}
		assert.NotEqual(t, err, nil)
	})

	t.Run("Should not create web platform credential without HTTPReferrerRestriction or IPRestriction", func(t *testing.T) {
		credentialManager := credential.Manager{}
		credentialManager.Platform = "web"
		_, err := credentialManager.CreateOne(dbConnection)
		if err == nil {
			utils.Error("[ERROR] Created a new credential without HTTPReferrerRestriction")
		}
		assert.NotEqual(t, err, nil)
	})

	t.Run("Should create web platform credential with HTTPReferrerRestriction", func(t *testing.T) {
		credentialManager := credential.Manager{}
		credentialManager.Platform = "web"
		credentialManager.HTTPReferrerRestriction = "*"
		_, err := credentialManager.CreateOne(dbConnection)
		if err != nil {
			utils.Error(fmt.Sprintf("[ERROR] failed to create web credential with http restriction: - %v", err.Message))
		}
		assert.Nil(t, err)
		forceDelete(&credentialManager)
	})

	t.Run("Should create web platform credential with IPRestriction", func(t *testing.T) {
		credentialManager := credential.Manager{}
		credentialManager.Platform = "web"
		credentialManager.IPRestriction = "*"
		_, err := credentialManager.CreateOne(dbConnection)
		if err != nil {
			utils.Error(fmt.Sprintf("[ERROR] failed to create web credential with ip restriction %v", err.Message))
		}
		assert.Nil(t, err)
		credentialManager.DeleteOne(dbConnection)
		forceDelete(&credentialManager)
	})

	t.Run("Should not create android platform credential without android package name restriction", func(t *testing.T) {
		credentialManager := credential.Manager{}
		credentialManager.Platform = "android"
		_, err := credentialManager.CreateOne(dbConnection)
		if err == nil {
			utils.Error("[ERROR] Created an android credential without package name restriction")
		}
		assert.NotEqual(t, err, nil)
	})

	t.Run("Should create android platform credential with android package name restriction", func(t *testing.T) {
		credentialManager := credential.Manager{}
		credentialManager.Platform = "android"
		credentialManager.AndroidPackageNameRestriction = "com.android.org"
		_, err := credentialManager.CreateOne(dbConnection)
		if err != nil {
			utils.Error(fmt.Sprintf("[ERROR] failed to create an android credential with package name restriction %v", err.Message))
		}
		assert.Nil(t, err)
		forceDelete(&credentialManager)
	})

	t.Run("Should not create ios platform credential without bundle id restriction", func(t *testing.T) {
		credentialManager := credential.Manager{}
		credentialManager.Platform = "ios"
		_, err := credentialManager.CreateOne(dbConnection)
		if err == nil {
			utils.Error("[ERROR] Created an ios credential bundle id restriction")
		}
		assert.NotEqual(t, err, nil)
	})

	t.Run("Should create ios platform credential with android package name restriction", func(t *testing.T) {
		credentialManager := credential.Manager{}
		credentialManager.Platform = "ios"
		credentialManager.IOSBundleIDRestriction = "com.ios.org"
		_, err := credentialManager.CreateOne(dbConnection)
		if err != nil {
			utils.Error(fmt.Sprintf("[ERROR] failed to create an ios credential with package name restriction %v", err.Message))
		}
		assert.Nil(t, err)
		forceDelete(&credentialManager)
	})

	t.Run("Should create server credential", func(t *testing.T) {
		credentialManager := credential.Manager{}
		credentialManager.Platform = "server"
		_, err := credentialManager.CreateOne(dbConnection)
		if err != nil {
			utils.Error("[ERROR] Failed to create a server credential" + err.Message)
		}

		assert.Nil(t, err)
		assert.NotEmpty(t, credentialManager.ApiSecret)
		assert.NotEmpty(t, credentialManager.ApiKey)
		forceDelete(&credentialManager)
	})

	t.Run("Should not update credential api key and secret", func(t *testing.T) {
		credentialManager := credential.Manager{}
		credentialManager.Platform = "server"
		_, err := credentialManager.CreateOne(dbConnection)
		if err != nil {
			utils.Error("[ERROR] Failed to create a new credential" + err.Message)
		}

		assert.Nil(t, err)

		credentialManager.ApiKey = "13455"
		_, updateErr := credentialManager.UpdateOne(dbConnection)
		if updateErr == nil {
			utils.Error("[ERROR] should not update credential key")
		}
		assert.NotEqual(t, updateErr, nil)
		forceDelete(&credentialManager)
	})

	t.Run("Update credential HTTPReferrerRestriction", func(t *testing.T) {
		credentialManager := credential.Manager{
			Platform:                "web",
			HTTPReferrerRestriction: "*",
		}
		_, err := credentialManager.CreateOne(dbConnection)
		if err != nil {
			utils.Error("[ERROR] Failed to create a new credential" + err.Message)
		}

		assert.Nil(t, err)

		credentialManager.HTTPReferrerRestriction = "http://google.com"
		_, updateError := credentialManager.UpdateOne(dbConnection)
		if updateError != nil {
			utils.Error(updateError.Error())
		}

		updatedCredential := credential.Manager{
			ID:   credentialManager.ID,
			UUID: credentialManager.UUID,
		}

		updatedCredential.GetOne(dbConnection)

		assert.Equal(t, updatedCredential.HTTPReferrerRestriction, credentialManager.HTTPReferrerRestriction)
		forceDelete(&credentialManager)
	})

	t.Run("Delete one credential", func(t *testing.T) {
		credentialManager := credential.Manager{
			Platform: credential.ServerPlatform,
		}
		otherCredentialManager := credential.Manager{
			Platform:      credential.WebPlatform,
			IPRestriction: "0.0.0.0",
		}
		_, err := credentialManager.CreateOne(dbConnection)
		_, otherCreateErr := otherCredentialManager.CreateOne(dbConnection)
		if err != nil {
			utils.Error("[ERROR] Failed to create a new credential" + err.Message)
		}
		if otherCreateErr != nil {
			utils.Error("[ERROR] Failed to create a new credential" + otherCreateErr.Message)
		}
		assert.Nil(t, err)
		assert.Nil(t, otherCreateErr)
		rowAffected, deleteErr := credentialManager.DeleteOne(dbConnection)
		assert.Nil(t, deleteErr)
		if deleteErr != nil {
			utils.Error(deleteErr.Error())
		}
		assert.Equal(t, rowAffected, 1)
		forceDelete(&otherCredentialManager)
	})

	t.Run("CredentialManager.List", func(t *testing.T) {
		credentialManager1 := credential.Manager{
			Platform: credential.ServerPlatform,
		}
		credentialManager2 := credential.Manager{
			Platform:      credential.WebPlatform,
			IPRestriction: "0.0.0.0",
		}
		credentialManager1.CreateOne(dbConnection)
		credentialManager2.CreateOne(dbConnection)

		credentialManager := credential.Manager{}
		credentials, err := credentialManager.GetAll(dbConnection, 0, 100, "date_created")
		assert.Nil(t, err)
		if err != nil {
			utils.Error(fmt.Sprintf("CredentialManager.List::Error::%v", err.Message))
		}

		assert.Equal(t, 2, len(credentials))
		forceDelete(&credentialManager1)
		forceDelete(&credentialManager2)
	})

	t.Run("Prevent deleting all credential", func(t *testing.T) {
		credentialManager1 := credential.Manager{
			Platform: credential.ServerPlatform,
		}
		credentialManager2 := credential.Manager{
			Platform:      credential.WebPlatform,
			IPRestriction: "0.0.0.0",
		}
		credentialManager1.CreateOne(dbConnection)
		credentialManager2.CreateOne(dbConnection)

		credentialManager := credential.Manager{}
		credentials, err := credentialManager.GetAll(dbConnection, 0, 100, "date_created")
		assert.Nil(t, err)
		if err != nil {
			utils.Error(err.Message)
		}

		for i := 0; i < len(credentials)-1; i++ {
			_, err := credentials[i].DeleteOne(dbConnection)
			assert.Equal(t, nil, err)
		}

		_, deleteCredentialError := credentials[len(credentials)-1].DeleteOne(dbConnection)
		assert.NotEqual(t, deleteCredentialError, nil)
		forceDelete(&credentialManager1)
		forceDelete(&credentialManager2)
	})
}
