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

type CredentialManager models.CredentialModel

func (credentialManager *CredentialManager) CreateOne(pool *utils.Pool) (string, error) {
	if len(credentialManager.HTTPReferrerRestriction) < 1 {
		return "", errors.New("credential should have at least one restriction set")
	}

	randomId := ksuid.New().String()
	hash := sha256.New()
	hash.Write([]byte(randomId))
	credentialManager.ApiKey = hex.EncodeToString(hash.Sum(nil))

	conn, err := pool.Acquire()
	defer pool.Release(conn)
	if err != nil {
		return "", err
	}
	db := conn.(*pg.DB)

	if _, err := db.Model(credentialManager).Insert(); err != nil {
		return "", err
	} else {
		return credentialManager.UUID, nil
	}
}

func (credentialManager *CredentialManager) GetOne(pool *utils.Pool) error {
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

func (credentialManager *CredentialManager) GetByAPIKey(pool *utils.Pool) error {
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

func (credentialManager *CredentialManager) Count(pool *utils.Pool) (int, *utils.GenericError) {
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

func (credentialManager *CredentialManager) GetAll(pool *utils.Pool, offset int, limit int, orderBy string) ([]CredentialManager, *utils.GenericError) {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	credentialManagers := []CredentialManager{}

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

func (credentialManager *CredentialManager) UpdateOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var credentialPlaceholder CredentialManager
	credentialPlaceholder.UUID = credentialManager.UUID
	err = credentialPlaceholder.GetOne(pool)
	if err != nil {
		return 0, err
	}

	if credentialPlaceholder.ApiKey != credentialManager.ApiKey && len(credentialManager.ApiKey) > 1 {
		return 0, errors.New("cannot update api key")
	}

	credentialManager.ApiKey = credentialPlaceholder.ApiKey
	credentialManager.DateCreated = credentialPlaceholder.DateCreated

	res, err := db.Model(credentialManager).Where("uuid = ?", credentialManager.UUID).Update(credentialManager)

	if err != nil {
		return 0, err
	}

	return res.RowsAffected(), nil
}

func (credentialManager *CredentialManager) DeleteOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	credentials := []CredentialManager{}

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
