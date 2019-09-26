package models

import (
	"encoding/json"
	"github.com/go-pg/pg"
	"github.com/segmentio/ksuid"
	"reflect"
)

type Project struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ID          string `json:"id"`
}

func (p *Project) SetId(id string) {
	p.ID = id
}

func (p *Project) CreateOne() (string, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	p.ID = ksuid.New().String()

	_, err := db.Model(p).Insert()
	if err != nil {
		return "", err
	}

	return p.ID, nil
}

func (p *Project) GetOne() error {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	err := db.Model(p).Where("id = ?", p.ID).Select()
	if err != nil {
		return err
	}

	return nil
}

func (p *Project) GetAll() (interface{}, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	var projects []Project

	err := db.Model(&projects).Select()
	if err != nil {
		return projects, err
	}

	vp := reflect.New(reflect.TypeOf(projects))
	vp.Elem().Set(reflect.ValueOf(projects))
	return vp.Interface(), nil
}

func (p *Project) UpdateOne() error {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	err := db.Update(p)
	if err != nil {
		return err
	}

	return nil
}

func (p *Project) DeleteOne() error {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	_, err := db.Model(p).Where("id = ?", p.ID).Delete()
	if err != nil {
		return err
	}

	return nil
}

func (p *Project) ToJson() []byte {
	data, err := json.Marshal(p)

	if err != nil {
		panic(err)
	}

	return data
}

func (p *Project) FromJson(body []byte) {
	err := json.Unmarshal(body, &p)

	if err != nil {
		panic(err)
	}
}
