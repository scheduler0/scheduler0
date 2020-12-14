package managers

import (
	"cron-server/server/src/models"
	"cron-server/server/src/utils"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/go-pg/pg"
	"github.com/segmentio/ksuid"
)

type CredentialManager models.CredentialModel

func (credentialManager *CredentialManager) CreateOne(pool *utils.Pool) (string, error) {
	if len(credentialManager.HTTPReferrerRestriction) < 1 {
		return "", errors.New("credential should have at least one restriction set")
	}

	credentialManager.ID = ksuid.New().String()

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
		return credentialManager.ID, nil
	}
}

func (credentialManager *CredentialManager) GetOne(pool *utils.Pool) error {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return err
	}

	db := conn.(*pg.DB)

	err = db.Model(credentialManager).Where("id = ?", credentialManager.ID).Select()
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

func (credentialManager *CredentialManager) GetAll(pool *utils.Pool, offset int, limit int, orderBy string) ([]CredentialManager, error) {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return []CredentialManager{}, err
	}

	credentials := []CredentialManager{}

	db := conn.(*pg.DB)

	err = db.Model(&credentials).
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()

	if err != nil {
		return nil, err
	}

	return credentials, nil
}

func (credentialManager *CredentialManager) UpdateOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var credentialPlaceholder CredentialManager
	credentialPlaceholder.ID = credentialManager.ID
	err = credentialPlaceholder.GetOne(pool)
	if err != nil {
		return 0, err
	}

	if credentialPlaceholder.ApiKey != credentialManager.ApiKey && len(credentialManager.ApiKey) > 1 {
		return 0, errors.New("cannot update api key")
	}

	credentialManager.ApiKey = credentialPlaceholder.ApiKey
	credentialManager.DateCreated = credentialPlaceholder.DateCreated

	res, err := db.Model(credentialManager).Where("id = ?", credentialManager.ID).Update(credentialManager)

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

	var credentials []CredentialManager

	err = db.Model(&credentials).Where("id != ?", "null").Select()
	if err != nil {
		return -1, err
	}

	if len(credentials) == 1 {
		err = errors.New("cannot delete all the credentials")
		return -1, err
	}

	r, err := db.Model(credentialManager).Where("id = ?", credentialManager.ID).Delete()
	if err != nil {
		return -1, err
	}

	return r.RowsAffected(), nil
}
