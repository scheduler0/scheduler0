package domains

import (
	"context"
	"cron-server/server/migrations"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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

func (c *CredentialDomain) GetOne(pool *migrations.Pool, ctx context.Context, query string, params interface{}) (int, error) {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)

	baseQuery := db.Model(c).Where(query, params)

	count, err := baseQuery.Count()
	if count < 1 {
		return 0, nil
	}

	if err != nil {
		return count, err
	}

	err = baseQuery.Select()
	if err != nil {
		return count, err
	}

	return count, nil
}

func (c *CredentialDomain) GetAll(pool *migrations.Pool, ctx context.Context, query string, offset int, limit int, orderBy string, params ...string) (int, []interface{}, error) {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return 0, []interface{}{}, err
	}

	var credentials []CredentialDomain

	db := conn.(*pg.DB)

	queryParameters := make([]interface{}, len(params))

	for i := 0; i < len(params); i++ {
		queryParameters[i] = params[i]
	}

	fmt.Println("query, values", query)
	fmt.Println("queryParameters", queryParameters)
	
	baseQuery := db.Model(&credentials).Where(query, queryParameters...)

	count, err := baseQuery.Count()
	if err != nil {
		return 0, []interface{}{}, err
	}

	fmt.Println("count", count)

	err = baseQuery.
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()

	if err != nil {
		return 0, []interface{}{}, err
	}

	var results = make([]interface{}, len(credentials))

	for i := 0; i < len(credentials); i++ {
		results[i] = credentials[i]
	}

	return count, results, nil
}

func (c *CredentialDomain) UpdateOne(pool *migrations.Pool, ctx context.Context) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var credentialPlaceholder CredentialDomain
	credentialPlaceholder.ID = c.ID
	_, err = credentialPlaceholder.GetOne(pool, ctx, "id = ?", credentialPlaceholder.ID)
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

func (c *CredentialDomain) DeleteOne(pool *migrations.Pool, ctx context.Context) (int, error) {
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
