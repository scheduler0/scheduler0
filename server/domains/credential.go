package domains

import (
	"context"
	"cron-server/server/migrations"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/go-pg/pg"
	"github.com/segmentio/ksuid"
	"time"
)

type CredentialDomain struct {
	ID                      string    `json:"id" pg:",notnull"`
	ApiKey                  string    `json:"api_key" pg:",notnull"`
	HTTPReferrerRestriction string    `json:"http_referrer_restriction" pg:",notnull"`
	DateCreated             time.Time `json:"date_created" pg:",notnull"`
}

func (c *CredentialDomain) CreateOne(pool *migrations.Pool, ctx *context.Context) (string, error) {
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

func (c *CredentialDomain) GetOne(pool *migrations.Pool, ctx *context.Context) (int, error) {
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

func (c *CredentialDomain) GetAll(pool *migrations.Pool, offset int, limit int, orderBy string) ([]CredentialDomain, error) {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return []CredentialDomain{}, err
	}

	credentials := []CredentialDomain{{}}

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

func (c *CredentialDomain) UpdateOne(pool *migrations.Pool, ctx *context.Context) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var credentialPlaceholder CredentialDomain
	credentialPlaceholder.ID = c.ID
	_, err = credentialPlaceholder.GetOne(pool, ctx)
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

func (c *CredentialDomain) DeleteOne(pool *migrations.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var credentials []CredentialDomain

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
