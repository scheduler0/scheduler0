package credential

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/go-pg/pg"
	"github.com/segmentio/ksuid"
	"net/http"
	"scheduler0/server/models"
	"scheduler0/utils"
)

type Manager models.CredentialModel


// CreateOne creates a single credential and returns the uuid
func (credentialManager *Manager) CreateOne(pool *utils.Pool) (string, *utils.GenericError) {

	if len(credentialManager.Platform) < 1 {
		return "", utils.HTTPGenericError(http.StatusBadRequest,"credential should have a platform")
	}

	if credentialManager.Platform != "server" &&
		credentialManager.Platform != "web" &&
		credentialManager.Platform != "ios" &&
		credentialManager.Platform != "android" {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "credential platform should be one of server, web, android, or ios")
	}

	switch credentialManager.Platform {
	case "android":
		if len(credentialManager.AndroidPackageNameRestriction) < 1 {
			return "", utils.HTTPGenericError(http.StatusBadRequest,"android credentials should have a package name restriction")
		}
	case "ios":
		if len(credentialManager.IOSBundleIDRestriction) < 1 {
			return "", utils.HTTPGenericError(http.StatusBadRequest,"ios credentials should have a bundle restriction")
		}
	case "web":
		if len(credentialManager.HTTPReferrerRestriction) < 1 && len(credentialManager.IPRestriction) < 1 {
			return "", utils.HTTPGenericError(http.StatusBadRequest,"web credentials should either an ip restriction or a url restriction")
		}
	}

	randomId := ksuid.New().String()
	hash := sha256.New()
	hash.Write([]byte(randomId))
	credentialManager.ApiKey = hex.EncodeToString(hash.Sum(nil))

	if credentialManager.Platform == "server" {
		hash := sha256.New()
		hash.Write([]byte(credentialManager.ApiKey))
		credentialManager.ApiSecret = hex.EncodeToString(hash.Sum(nil))
	}

	conn, err := pool.Acquire()
	defer pool.Release(conn)
	if err != nil {
		return "", utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	db := conn.(*pg.DB)

	if _, err := db.Model(credentialManager).Insert(); err != nil {
		return "", utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	} else {
		return credentialManager.UUID, nil
	}
}

// GetOne returns a single credential
func (credentialManager *Manager) GetOne(pool *utils.Pool) error {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return err
	}

	db := conn.(*pg.DB)

	err = db.Model(credentialManager).Where("uuid = ?", credentialManager.UUID).Select()
	if err != nil {
		return err
	}

	return nil
}

// GetByAPIKey returns a credential with the matching api key
func (credentialManager *Manager) GetByAPIKey(pool *utils.Pool) error {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return err
	}

	db := conn.(*pg.DB)

	err = db.Model(credentialManager).Where("api_key = ?", credentialManager.ApiKey).Select()
	if err != nil {
		return err
	}

	return nil
}

// Count returns total number of credential
func (credentialManager *Manager) Count(pool *utils.Pool) (int, *utils.GenericError) {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	db := conn.(*pg.DB)

	count, err := db.Model(credentialManager).Count()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return count, nil
}

// GetAll returns a paginated set of credentials
func (credentialManager *Manager) GetAll(pool *utils.Pool, offset int, limit int, orderBy string) ([]Manager, *utils.GenericError) {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	credentialManagers := []Manager{}

	db := conn.(*pg.DB)

	err = db.Model(&credentialManagers).
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
func (credentialManager *Manager) UpdateOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	credentialPlaceholder := Manager{
		UUID: credentialManager.UUID,
	}
	err = credentialPlaceholder.GetOne(pool)
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

	res, err := db.Model(credentialManager).Where("uuid = ?", credentialManager.UUID).Update(credentialManager)

	if err != nil {
		return 0, err
	}

	return res.RowsAffected(), nil
}

// DeleteOne deletes a single credential
func (credentialManager *Manager) DeleteOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	credentials := []Manager{}

	count, err := db.Model(&credentials).Count()
	if err != nil {
		return -1, err
	}

	if count == 1 {
		err = errors.New("cannot delete all the credentials")
		return -1, err
	}

	r, err := db.Model(credentialManager).Where("uuid = ?", credentialManager.UUID).Delete()
	if err != nil {
		return -1, err
	}

	return r.RowsAffected(), nil
}
