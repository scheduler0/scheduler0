package models

import (
	"encoding/json"
	"errors"
	"github.com/go-pg/pg"
	"github.com/segmentio/ksuid"
	"reflect"
	"time"
)

type Project struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ID          string    `json:"id"`
	DateCreated time.Time `json:"date_created"`
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
	data, err := projectWithName.GetAll("name LIKE ?", p.Name+"%")

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
	p.DateCreated = time.Now().UTC()

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

func (p *Project) GetAll(query string, params ...string) ([]interface{}, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	ip := make([]interface{}, len(params))

	for i := 0; i < len(params); i++ {
		ip[i] = params[i]
	}

	var projects []Project
	err := db.Model(&projects).Where(query, ip...).Select()
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

	savedProject := Project{ID: p.ID}

	if err := savedProject.GetOne("id = ?", savedProject.ID); err != nil {
		return err
	}

	if len(savedProject.ID) < 1 {
		return errors.New("project does not exist")
	}

	if savedProject.Name != p.Name {
		if projectsWithSameName, err := p.GetAll("name = ? AND id != ?", p.Name, p.ID); err != nil {
			return err
		} else {
			if len(projectsWithSameName) > 0 {
				return errors.New("project with same name exits")
			}

			if err := db.Update(p); err != nil {
				return err
			}
		}
	} else {
		if err := db.Update(p); err != nil {
			return err
		}
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

func (p *Project) SearchToQuery(search [][]string) (string, []string) {
	var queries []string
	var values []string
	var query string

	if len(search) < 1 || search[0] == nil {
		return query, values
	}

	for i := 0; i < len(search); i++ {
		if search[i][0] == "id" {
			queries = append(queries, "id = ?")
			values = append(values, search[i][1])
		}

		if search[i][0] == "name" {
			queries = append(queries, "name LIKE ?")
			values = append(values, "%"+search[i][1]+"%")
		}

		if search[i][0] == "description" {
			queries = append(queries, "description LIKE ?")
			values = append(values, "%"+search[i][1]+"%")
		}
	}

	for i := 0; i < len(queries); i++ {
		if i != 0 {
			query += " AND " + queries[i]
		} else {
			query = queries[i]
		}
	}

	return query, values
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
