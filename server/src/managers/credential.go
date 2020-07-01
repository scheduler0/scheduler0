package managers

import (
	"cron-server/server/src/db"
	"cron-server/server/src/models"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/go-pg/pg"
	"github.com/segmentio/ksuid"
	"time"
)

type CredentialManager models.CredentialModel

func (c *CredentialManager) CreateOne(pool *db.Pool) (string, error) {
	if len(c.HTTPReferrerRestriction) < 1 {
		return "", errors.New("credential should have at least one restriction set")
	}

	c.DateCreated = time.Now().UTC()
	c.ID = ksuid.New().String()

	randomId := ksuid.New().String()
	hash := sha256.New()
	hash.Write([]byte(randomId))
	c.ApiKey = hex.EncodeToString(hash.Sum(nil))

	conn, err := pool.Acquire()
	defer pool.Release(conn)
	if err != nil {
		return "", err
	}
	db := conn.(*pg.DB)

	if _, err := db.Model(c).Insert(); err != nil {
		return "", err
	} else {
		return c.ID, nil
	}
}

func (c *CredentialManager) GetOne(pool *db.Pool) (int, error) {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)

	baseQuery := db.Model(c)

	count, err := baseQuery.Count()
	if count < 1 {
		return 0, nil
	}

	if err != nil {
		return count, err
	}

	err = baseQuery.Select(&c)
	if err != nil {
		return count, err
	}

	return count, nil
}

func (c *CredentialManager) GetAll(pool *db.Pool, offset int, limit int, orderBy string) ([]CredentialManager, error) {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return []CredentialManager{}, err
	}

	credentials := []CredentialManager{{}}

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

func (c *CredentialManager) UpdateOne(pool *db.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var credentialPlaceholder CredentialManager
	credentialPlaceholder.ID = c.ID
	_, err = credentialPlaceholder.GetOne(pool)
	if err != nil {
		return 0, err
	}

	if credentialPlaceholder.ApiKey != c.ApiKey && len(c.ApiKey) > 1 {
		return 0, errors.New("cannot update api key")
	}

	c.ApiKey = credentialPlaceholder.ApiKey
	c.DateCreated = credentialPlaceholder.DateCreated

	res, err := db.Model(&c).Update(c)

	if err != nil {
		return 0, err
	}

	return res.RowsAffected(), nil
}

func (c *CredentialManager) DeleteOne(pool *db.Pool) (int, error) {
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

	r, err := db.Model(c).Where("id = ?", c.ID).Delete()
	if err != nil {
		return -1, err
	}

	return r.RowsAffected(), nil
}
