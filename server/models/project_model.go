package models

import (
	"context"
	"cron-server/server/repository"
	"encoding/json"
	"errors"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"github.com/segmentio/ksuid"
	"time"
)

type Project struct {
	Name        string    `json:"name" pg:",notnull"`
	Description string    `json:"description" pg:",notnull"`
	ID          string    `json:"id" pg:",notnull"`
	DateCreated time.Time `json:"date_created" pg:",notnull"`
}

func (p *Project) SetId(id string) {
	p.ID = id
}

func (p *Project) CreateOne(pool *repository.Pool, ctx context.Context) (string, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return "", err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if len(p.Name) < 1 {
		err := errors.New("name field is required")
		return "", err
	}

	if len(p.Description) < 1 {
		err := errors.New("description field is required")
		return "", err
	}

	var projectWithName = Project{}
	err = projectWithName.GetOne(pool, ctx, "name LIKE ?", []string{p.Name+"%"})
	if err != nil {
		return "", err
	}

	if err == nil && len(projectWithName.ID) > 0 {
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

func (p *Project) GetOne(pool *repository.Pool, ctx context.Context, query string, params interface{}) error {
	conn, err := pool.Acquire()
	if err != nil {
		return err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	err = db.Model(p).Where(query, params).Select()
	if err != nil {
		return err
	}

	return nil
}

func (p *Project) GetAll(pool *repository.Pool, ctx context.Context, query string, offset int, limit int, orderBy string, params ...string) ([]interface{}, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return []interface{}{}, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	ip := make([]interface{}, len(params))

	for i := 0; i < len(params); i++ {
		ip[i] = params[i]
	}

	var projects []Project
	err = db.Model(&projects).
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

	var results = make([]interface{}, len(projects))

	for i := 0; i < len(projects); i++ {
		results[i] = projects[i]
	}

	return results, nil
}

func (p *Project) UpdateOne(pool *repository.Pool, ctx context.Context) error {
	conn, err := pool.Acquire()
	if err != nil {
		return err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	savedProject := Project{ID: p.ID}

	if err := savedProject.GetOne(pool, ctx, "id = ?", savedProject.ID); err != nil {
		return err
	}

	if len(savedProject.ID) < 1 {
		return errors.New("project does not exist")
	}

	if savedProject.Name != p.Name {
		var filters = []string{p.Name, p.ID}

		var projectWithSimilarName = Project{}

		err := projectWithSimilarName.GetOne(pool, ctx, "name = ? AND id != ?", filters)
		if  err != nil {
			return err
		}

		if len(projectWithSimilarName.ID) < 1 {
			return errors.New("project with same name exits")
		}
	}

	if err := db.Update(p); err != nil {
		return err
	}

	return nil
}

func (p *Project) DeleteOne(pool *repository.Pool, ctx context.Context) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var jobs []Job

	err = db.Model(&jobs).Where("project_id = ?", p.ID).Select()
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

	if len(query) < 1 && len(values) < 1 {
		values = append(values, "null")
		return "id != ?", values
	}

	return query, values
}

func (p *Project) ToJson() ([]byte, error) {
	if data, err := json.Marshal(p); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

func (p *Project) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &p); err != nil {
		return err
	}
	return nil
}
