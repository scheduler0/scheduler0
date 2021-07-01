package credential

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/go-pg/pg"
	"github.com/segmentio/ksuid"
	"net/http"
	"scheduler0/server/models"
	"scheduler0/utils"
)

// Manager Credential
type Manager models.CredentialModel

const (
	AndroidPlatform = "android"
	WebPlatform     = "web"
	IOSPlatform     = "ios"
	ServerPlatform  = "server"
)

func getRandomSha256() string {
	randomId := ksuid.New().String()
	hash := sha256.New()
	hash.Write([]byte(randomId))
	return hex.EncodeToString(hash.Sum(nil))
}

// CreateOne creates a single credential and returns the uuid
func (credentialManager *Manager) CreateOne(dbConnection *pg.DB) (string, *utils.GenericError) {
	if len(credentialManager.Platform) < 1 {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "credential should have a platform")
	}

	if credentialManager.Platform != AndroidPlatform &&
		credentialManager.Platform != WebPlatform &&
		credentialManager.Platform != IOSPlatform &&
		credentialManager.Platform != ServerPlatform {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "credential platform should be one of server, web, android, or ios")
	}

	switch credentialManager.Platform {
	case AndroidPlatform:
		if len(credentialManager.AndroidPackageNameRestriction) < 1 {
			return "", utils.HTTPGenericError(http.StatusBadRequest, "android credentials should have a package name restriction")
		}
	case IOSPlatform:
		if len(credentialManager.IOSBundleIDRestriction) < 1 {
			return "", utils.HTTPGenericError(http.StatusBadRequest, "ios credentials should have a bundle restriction")
		}
	case WebPlatform:
		if len(credentialManager.HTTPReferrerRestriction) < 1 && len(credentialManager.IPRestriction) < 1 {
			return "", utils.HTTPGenericError(http.StatusBadRequest, "web credentials should either an ip restriction or a url restriction")
		}
	}

	configs := utils.GetScheduler0Configurations()

	credentialManager.ApiKey = utils.Encrypt(getRandomSha256(), configs.SecretKey)

	if credentialManager.Platform == ServerPlatform {
		credentialManager.ApiSecret = utils.Encrypt(getRandomSha256(), configs.SecretKey)
	}


	if _, err := dbConnection.Model(credentialManager).Insert(); err != nil {
		return "", utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	} else {
		return credentialManager.UUID, nil
	}
}

// GetOne returns a single credential
func (credentialManager *Manager) GetOne(dbConnection *pg.DB) error {
	err := dbConnection.Model(credentialManager).Where("uuid = ?", credentialManager.UUID).Select()
	if err != nil {
		return err
	}

	return nil
}

// GetByAPIKey returns a credential with the matching api key
func (credentialManager *Manager) GetByAPIKey(dbConnection *pg.DB) *utils.GenericError {


	

	count, err := dbConnection.Model(credentialManager).Where("api_key = ?", credentialManager.ApiKey).Count()
	if count < 1 {
		return utils.HTTPGenericError(http.StatusNotFound, fmt.Sprintf("cannot find api_key=%v", credentialManager.ApiKey))
	}

	err = dbConnection.Model(credentialManager).Where("api_key = ?", credentialManager.ApiKey).Select()
	if err != nil {
		return utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return nil
}

// Count returns total number of credential
func (credentialManager *Manager) Count(dbConnection *pg.DB) (int, *utils.GenericError) {
	count, err := dbConnection.Model(credentialManager).Count()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return count, nil
}

// GetAll returns a paginated set of credentials
func (credentialManager *Manager) GetAll(dbConnection *pg.DB, offset int, limit int, orderBy string) ([]Manager, *utils.GenericError) {
	credentialManagers := []Manager{}

	err := dbConnection.Model(&credentialManagers).
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()

	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return credentialManagers, nil
}

// UpdateOne updates a single credential
func (credentialManager *Manager) UpdateOne(dbConnection *pg.DB) (int, error) {
	credentialPlaceholder := Manager{
		UUID: credentialManager.UUID,
	}
	err := credentialPlaceholder.GetOne(dbConnection)
	if err != nil {
		return 0, err
	}

	if credentialPlaceholder.ApiKey != credentialManager.ApiKey && len(credentialManager.ApiKey) > 1 {
		return 0, errors.New("cannot update api key")
	}

	if credentialPlaceholder.ApiSecret != credentialManager.ApiSecret && len(credentialManager.ApiSecret) > 1 {
		return 0, errors.New("cannot update api secret")
	}

	credentialManager.ApiKey = credentialPlaceholder.ApiKey
	credentialManager.ApiSecret = credentialPlaceholder.ApiSecret
	credentialManager.DateCreated = credentialPlaceholder.DateCreated

	res, err := dbConnection.Model(credentialManager).Where("uuid = ?", credentialManager.UUID).Update(credentialManager)

	if err != nil {
		return 0, err
	}

	return res.RowsAffected(), nil
}

// DeleteOne deletes a single credential
func (credentialManager *Manager) DeleteOne(dbConnection *pg.DB) (int, error) {
	credentials := []Manager{}

	count, err := dbConnection.Model(&credentials).Count()
	if err != nil {
		return -1, err
	}

	if count == 1 {
		err = errors.New("cannot delete all the credentials")
		return -1, err
	}

	r, err := dbConnection.Model(credentialManager).Where("uuid = ?", credentialManager.UUID).Delete()
	if err != nil {
		return -1, err
	}

	return r.RowsAffected(), nil
}
