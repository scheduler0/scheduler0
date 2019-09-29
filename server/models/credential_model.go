package models

import (
	"encoding/json"
	"errors"
	"github.com/go-pg/pg"
	"github.com/segmentio/ksuid"
	"time"
)

type Credential struct {
	ID                      string    `json:"id"`
	ApiKey                  string    `json:"api_key"`
	ApiSecret               string    `json:"api_secret"`
	PublicKey               string    `json:"public_key"`
	HTTPReferrerRestriction string    `json:"http_referrer_restriction"`
	IPAddressRestriction    string    `json:"ip_address_restriction"`
	DateCreated             time.Time `json:"date_created"`
}

func (c *Credential) SetId(id string) {
	c.ID = id
}

func (c *Credential) CreateOne() (string, error) {
	if len(c.ApiKey) < 1 {
		return "", errors.New("api key is required")
	}

	if len(c.ApiSecret) < 1 {
		return "", errors.New("api secret is required")
	}

	if len(c.PublicKey) < 1 {
		return "", errors.New("public key is required")
	}

	if len(c.HTTPReferrerRestriction) < 1 && len(c.IPAddressRestriction) < 1 {
		return "", errors.New("credential should have at least one restriction set")
	}

	c.DateCreated = time.Now().UTC()
	c.ID = ksuid.New().String()

	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	if _, err := db.Model(c).Insert(); err != nil {
		return "", err
	} else {
		return c.ID, nil
	}
}

func (c *Credential) ToJson() []byte {
	if data, err := json.Marshal(c); err != nil {
		panic(err)
	} else {
		return data
	}
}

func (c *Credential) FromJson(body []byte) {
	if err := json.Unmarshal(body, &c); err != nil {
		panic(err)
	}
}
