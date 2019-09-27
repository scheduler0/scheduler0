package models

import (
	"encoding/json"
	"errors"
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

	if len(p.Name) < 1 {
		err := errors.New("name field is required")
		return "", err
	}

	if len(p.Description) < 1 {
		err := errors.New("description field is required")
		return "", err
	}

	var projectWithName = Project{}
	data, err := projectWithName.GetAll("name LIKE ?", p.Name)

	vd := reflect.ValueOf(data)
	projectsWithName := make([]Project, vd.Len())

	for i := 0; i < vd.Len(); i++ {
		projectsWithName[i] = vd.Index(i).Interface().(Project)
	}

	if err == nil && len(projectsWithName) > 0 {
		err := errors.New("projects exits with the same name")
		return "", err
	}

	p.ID = ksuid.New().String()

	_, err = db.Model(p).Insert()
	if err != nil {
		return "", err
	}

	return p.ID, nil
}

func (p *Project) GetOne(query string, params interface{}) error {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	err := db.Model(p).Where(query, params).Select()
	if err != nil {
		return err
	}

	return nil
}

func (p *Project) GetAll(query string, params interface{}) ([]interface{}, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	var projects []Project
	err := db.Model(&projects).Where(query, params).Select()
	if err != nil {
		return []interface{}{}, err
	}

	var results = make([]interface{}, len(projects))

	for i := 0; i < len(projects); i++ {
		results[i] = projects[i]
	}

	return results, nil
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

func (p *Project) DeleteOne() (int, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	var jobs []Job

	err := db.Model(&jobs).Where("project_id = ?", p.ID).Select()
	if err != nil {
		return -1, err
	}

	if len(jobs) > 0 {
		err = errors.New("cannot delete projects with jobs")
		return -1, err
	}

	r, err := db.Model(p).Where("id = ?", p.ID).Delete()
	if err != nil {
		return -1, err
	}

	return r.RowsAffected(), nil
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
