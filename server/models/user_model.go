package models

import (
	"encoding/json"
	"errors"
	"github.com/go-pg/pg"
	"github.com/segmentio/ksuid"
	"regexp"
	"time"
)

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Password    string    `json:"password"`
	DateCreated time.Time `json:"date_created"`
}

var emailRegexp = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

func (u *User) SetId(id string) {
	u.ID = id
}

func (u *User) CreateOne() (string, error) {
	if !emailRegexp.MatchString(u.Email) {
		return "", errors.New("email is not valid")
	}

	if len(u.Password) < 6 {
		return "", errors.New("password should be at least six characters")
	}

	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	u.ID = ksuid.New().String()
	u.DateCreated = time.Now().UTC()

	if _, err := db.Model(u).Insert(); err != nil {
		return "", err
	}

	return u.ID, nil
}

func (u *User) ToJson() []byte {
	if data, err := json.Marshal(u); err != nil {
		panic(err)
	} else {
		return data
	}
}

func (u *User) FromJson(body []byte) {
	if err := json.Unmarshal(body, &u); err != nil {
		panic(err)
	}
}
