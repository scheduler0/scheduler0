package repository_test

import (
	_ "github.com/mattn/go-sqlite3"
)

//
//func Test_CredentialRepo(t *testing.T) {
//
//	utils.SetTestScheduler0Configurations()
//	db.TeardownTestDB()
//	db.PrepareTestDB()
//	dbConn := db.GetTestDBConnection()
//	store := store2.Store{
//		SqliteDB: dbConn,
//	}
//	credentialRepo := repository2.NewCredentialRepo(&store)
//
//	forceDelete := func(credentialRepo *models.CredentialModel) {
//		dbConn := db.GetTestDBConnection()
//		deleteQuery := sq.Delete(repository2.JobsTableName).Where("id = ?", credentialRepo.ID).RunWith(dbConn)
//		_, deleteErr := deleteQuery.Exec()
//		if deleteErr != nil {
//			log.Fatalln(deleteErr)
//		}
//	}
//
//	t.Run("Should not create a credential without a platform", func(t *testing.T) {
//		credentialModel := models.CredentialModel{}
//		_, err := credentialRepo.CreateOne(credentialModel)
//		if err == nil {
//			utils.Error("[ERROR] Created a new credential without platform")
//		}
//		assert.NotEqual(t, err, nil)
//	})
//
//	t.Run("Should not create web platform credential without HTTPReferrerRestriction or IPRestriction", func(t *testing.T) {
//		credentialModel := models.CredentialModel{}
//		credentialModel.Platform = "web"
//		_, err := credentialRepo.CreateOne(credentialModel)
//		if err == nil {
//			utils.Error("[ERROR] Created a new credential without HTTPReferrerRestriction")
//		}
//		assert.NotEqual(t, err, nil)
//	})
//
//	t.Run("Should create web platform credential with HTTPReferrerRestriction", func(t *testing.T) {
//		e, err := os.Executable()
//		utils.CheckErr(err)
//		ppath := path.Dir(e)
//		fmt.Println(ppath)
//
//		credentialModel := models.CredentialModel{}
//
//		credentialModel.Platform = "web"
//		credentialModel.HTTPReferrerRestriction = "*"
//		_, createErr := credentialRepo.CreateOne(credentialModel)
//		if createErr != nil {
//			utils.Error(fmt.Sprintf("[ERROR] failed to create web credential with http restriction: - %v", createErr.Message))
//		}
//		assert.Nil(t, createErr)
//		forceDelete(&credentialModel)
//	})
//
//	t.Run("Should create web platform credential with IPRestriction", func(t *testing.T) {
//		credentialModel := models.CredentialModel{}
//
//		credentialModel.Platform = "web"
//		credentialModel.IPRestriction = "*"
//		_, err := credentialRepo.CreateOne(credentialModel)
//		if err != nil {
//			utils.Error(fmt.Sprintf("[ERROR] failed to create web credential with ip restriction %v", err.Message))
//		}
//		assert.Nil(t, err)
//		_, err2 := credentialRepo.DeleteOneByID(credentialModel)
//		assert.Nil(t, err2)
//		forceDelete(&credentialModel)
//	})
//
//	t.Run("Should not create android platform credential without android package name restriction", func(t *testing.T) {
//		credentialModel := models.CredentialModel{}
//		credentialModel.Platform = "android"
//		_, err := credentialRepo.CreateOne(credentialModel)
//		if err == nil {
//			utils.Error("[ERROR] Created an android credential without package name restriction")
//		}
//		assert.NotEqual(t, err, nil)
//	})
//
//	t.Run("Should create android platform credential with android package name restriction", func(t *testing.T) {
//		credentialModel := models.CredentialModel{}
//
//		credentialModel.Platform = "android"
//		credentialModel.AndroidPackageNameRestriction = "com.android.org"
//		_, err := credentialRepo.CreateOne(credentialModel)
//		if err != nil {
//			utils.Error(fmt.Sprintf("[ERROR] failed to create an android credential with package name restriction %v", err.Message))
//		}
//		assert.Nil(t, err)
//		forceDelete(&credentialModel)
//	})
//
//	t.Run("Should not create ios platform credential without bundle id restriction", func(t *testing.T) {
//		credentialModel := models.CredentialModel{}
//		credentialModel.Platform = "ios"
//		_, err := credentialRepo.CreateOne(credentialModel)
//		if err == nil {
//			utils.Error("[ERROR] Created an ios credential bundle id restriction")
//		}
//		assert.NotEqual(t, err, nil)
//	})
//
//	t.Run("Should create ios platform credential with android package name restriction", func(t *testing.T) {
//		credentialModel := models.CredentialModel{}
//
//		credentialModel.Platform = "ios"
//		credentialModel.IOSBundleIDRestriction = "com.ios.org"
//		_, err := credentialRepo.CreateOne(credentialModel)
//		if err != nil {
//			utils.Error(fmt.Sprintf("[ERROR] failed to create an ios credential with package name restriction %v", err.Message))
//		}
//		assert.Nil(t, err)
//		forceDelete(&credentialModel)
//	})
//
//	t.Run("Should create server credential", func(t *testing.T) {
//		credentialModel := models.CredentialModel{}
//
//		credentialModel.Platform = "server"
//		_, err := credentialRepo.CreateOne(credentialModel)
//		if err != nil {
//			utils.Error("[ERROR] Failed to create a server credential" + err.Message)
//		}
//
//		assert.Nil(t, err)
//		assert.NotEmpty(t, credentialModel.ApiSecret)
//		assert.NotEmpty(t, credentialModel.ApiKey)
//		forceDelete(&credentialModel)
//	})
//
//	t.Run("Should not update credential api key and secret", func(t *testing.T) {
//		credentialModel := models.CredentialModel{}
//
//		credentialModel.Platform = "server"
//		_, err := credentialRepo.CreateOne(credentialModel)
//		if err != nil {
//			utils.Error("[ERROR] Failed to create a new credential" + err.Message)
//		}
//
//		assert.Nil(t, err)
//
//		credentialModel.ApiKey = "13455"
//		_, updateErr := credentialRepo.UpdateOneByID(credentialModel)
//		if updateErr == nil {
//			utils.Error("[ERROR] should not update credential key")
//		}
//		assert.NotEqual(t, updateErr, nil)
//		forceDelete(&credentialModel)
//	})
//
//	t.Run("Update credential HTTPReferrerRestriction", func(t *testing.T) {
//		credentialModel := models.CredentialModel{
//			Platform:                "web",
//			HTTPReferrerRestriction: "*",
//		}
//		_, err := credentialRepo.CreateOne(credentialModel)
//		if err != nil {
//			utils.Error("[ERROR] Failed to create a new credential" + err.Message)
//		}
//
//		assert.Nil(t, err)
//
//		credentialModel.HTTPReferrerRestriction = "http://google.com"
//		_, updateError := credentialRepo.UpdateOneByID(credentialModel)
//		if updateError != nil {
//			utils.Error(updateError.Error())
//		}
//
//		updatedCredential := models.CredentialModel{
//			ID: credentialModel.ID,
//		}
//
//		err2 := credentialRepo.GetOneID(&updatedCredential)
//		assert.Nil(t, err2)
//
//		assert.Equal(t, updatedCredential.HTTPReferrerRestriction, credentialModel.HTTPReferrerRestriction)
//		forceDelete(&credentialModel)
//	})
//
//	t.Run("Delete one credential", func(t *testing.T) {
//		credentialModel := models.CredentialModel{
//			Platform: repository2.ServerPlatform,
//		}
//		otherCredentialModel := models.CredentialModel{
//			Platform:      repository2.WebPlatform,
//			IPRestriction: "0.0.0.0",
//		}
//		_, err := credentialRepo.CreateOne(credentialModel)
//		_, otherCreateErr := credentialRepo.CreateOne(otherCredentialModel)
//		if err != nil {
//			utils.Error("[ERROR] Failed to create a new credential" + err.Message)
//		}
//		if otherCreateErr != nil {
//			utils.Error("[ERROR] Failed to create a new credential" + otherCreateErr.Message)
//		}
//		assert.Nil(t, err)
//		assert.Nil(t, otherCreateErr)
//		rowAffected, deleteErr := credentialRepo.DeleteOneByID(credentialModel)
//		assert.Nil(t, deleteErr)
//		if deleteErr != nil {
//			utils.Error(deleteErr.Error())
//		}
//		assert.Equal(t, rowAffected, int64(1))
//		forceDelete(&otherCredentialModel)
//	})
//
//	t.Run("CredentialRepo.List", func(t *testing.T) {
//		credentialModel1 := models.CredentialModel{
//			Platform: repository2.ServerPlatform,
//		}
//		credentialModel2 := models.CredentialModel{
//			Platform:      repository2.WebPlatform,
//			IPRestriction: "0.0.0.0",
//		}
//		_, err := credentialRepo.CreateOne(credentialModel1)
//		assert.Nil(t, err)
//
//		_, err2 := credentialRepo.CreateOne(credentialModel2)
//		assert.Nil(t, err2)
//
//		credentials, err := credentialRepo.List(0, 100, "date_created")
//		assert.Nil(t, err)
//		if err != nil {
//			utils.Error(fmt.Sprintf("CredentialRepo.List::Error::%v", err.Message))
//		}
//
//		assert.Equal(t, 2, len(credentials))
//
//		forceDelete(&credentialModel1)
//		forceDelete(&credentialModel2)
//	})
//
//	t.Run("Prevent deleting all credential", func(t *testing.T) {
//		credentialModel1 := models.CredentialModel{
//			Platform: repository2.ServerPlatform,
//		}
//		credentialModel2 := models.CredentialModel{
//			Platform:      repository2.WebPlatform,
//			IPRestriction: "0.0.0.0",
//		}
//		_, err := credentialRepo.CreateOne(credentialModel1)
//		assert.Nil(t, err)
//
//		_, err2 := credentialRepo.CreateOne(credentialModel2)
//		assert.Nil(t, err2)
//
//		credentials, err := credentialRepo.List(0, 100, "date_created")
//		assert.Nil(t, err)
//		if err != nil {
//			utils.Error(err.Message)
//		}
//
//		for i := 0; i < len(credentials)-1; i++ {
//			_, delErr := credentialRepo.DeleteOneByID(credentials[i])
//			assert.Equal(t, nil, delErr)
//		}
//
//		_, deleteCredentialError := credentialRepo.DeleteOneByID(credentialModel2)
//		assert.NotEqual(t, deleteCredentialError, nil)
//		forceDelete(&credentialModel1)
//		forceDelete(&credentialModel2)
//	})
//
//	t.Cleanup(func() {
//		db.TeardownTestDB()
//	})
//}
