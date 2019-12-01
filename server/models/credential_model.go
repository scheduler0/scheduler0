package models

import (
	"context"
	"cron-server/server/repository"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"github.com/segmentio/ksuid"
	"time"
)

type Credential struct {
	ID                      string    `json:"id" pg:",notnull"`
	ApiKey                  string    `json:"api_key" pg:",notnull"`
	HTTPReferrerRestriction string    `json:"http_referrer_restriction" pg:",notnull"`
	DateCreated             time.Time `json:"date_created" pg:",notnull"`
}

func (c *Credential) SetId(id string) {
	c.ID = id
}

func (c *Credential) CreateOne(pool *repository.Pool, ctx context.Context) (string, error) {
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

func (c *Credential) GetOne(pool *repository.Pool, ctx context.Context, query string, params interface{}) error {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return err
	}

	db := conn.(*pg.DB)
	err = db.Model(c).Where(query, params).Select()
	if err != nil {
		return err
	}

	return nil
}

func (c *Credential) GetAll(pool *repository.Pool, ctx context.Context, query string, offset int, limit int, orderBy string, params ...string) ([]interface{}, error) {
	conn, err := pool.Acquire()
	defer pool.Release(conn)

	if err != nil {
		return []interface{}{}, err
	}

	var credentials []Credential

	db := conn.(*pg.DB)
	err = db.Model(&credentials).
		WhereGroup(func(query *orm.Query) (query2 *orm.Query, e error) {
			for i := 0; i < len(params); i++ {
				query.WhereOr(params[i])
			}
			return  query, nil
		}).
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()
	if err != nil {
		return []interface{}{}, err
	}

	var results = make([]interface{}, len(credentials))

	for i := 0; i < len(credentials); i++ {
		results[i] = credentials[i]
	}

	return results, nil
}

func (c *Credential) UpdateOne(pool *repository.Pool, ctx context.Context) error {
	conn, err := pool.Acquire()
	if err != nil {
		return err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var credentialPlaceholder Credential
	credentialPlaceholder.ID = c.ID
	err = credentialPlaceholder.GetOne(pool, ctx, "id = ?", credentialPlaceholder.ID)

	if credentialPlaceholder.ApiKey != c.ApiKey && len(c.ApiKey) > 1 {
		return errors.New("cannot update api key")
	}

	c.ApiKey = credentialPlaceholder.ApiKey
	c.DateCreated = credentialPlaceholder.DateCreated

	if err = db.Update(c); err != nil {
		return err
	}

	return nil
}

func (c *Credential) DeleteOne(pool *repository.Pool, ctx context.Context) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var credentials []Credential

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

func (c *Credential) SearchToQuery([][]string) (string, []string) {
	return "api_key != ?", []string{"null"}
}

func (c *Credential) ToJson() ([]byte, error) {
	if data, err := json.Marshal(c); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

func (c *Credential) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &c); err != nil {
		return err
	}
	return nil
}
