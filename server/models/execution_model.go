package models

import (
	"context"
	"cron-server/server/repository"
	"encoding/json"
	"errors"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"time"
)

type Execution struct {
	ID          string    `json:"id"`
	JobId       string    `json:"job_id"`
	StatusCode  string    `json:"status_code"`
	Timeout     uint64    `json:"timeout"`
	Response    string    `json:"response"`
	Token    	string    `json:"token"`
	DateCreated time.Time `json:"date_created"`
}

func (exec *Execution) SetId(id string) {
	exec.ID = id
}

func (exec *Execution) CreateOne(pool *repository.Pool, ctx context.Context) (string, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return "", err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if len(exec.JobId) < 1 {
		err := errors.New("job id is not sets")
		return "", err
	}

	jobWithId := Job{ID: exec.JobId}

	if err := jobWithId.GetOne(pool, ctx, "id = ?", exec.JobId); err != nil {
		return "", errors.New("job with id does not exist")
	}

	if _, err := db.Model(exec).Insert(); err != nil {
		return "", err
	}

	return exec.ID, nil
}

func (exec *Execution) GetOne(pool *repository.Pool, ctx context.Context, query string, params interface{}) error {
	conn, err := pool.Acquire()
	if err != nil {
		return err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if err := db.Model(exec).Where(query, params).Select(); err != nil {
		return err
	}

	return nil
}

func (exec *Execution) GetAll(pool *repository.Pool, ctx context.Context, query string, offset int, limit int, orderBy string, params ...string) ([]interface{}, error) {
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

	var execs []Execution

	if err := db.
		Model(&execs).
		WhereGroup(func(query *orm.Query) (query2 *orm.Query, e error) {
			for i := 0; i < len(params); i++ {
				query.WhereOr(params[i])
			}
			return  query, nil
		}).
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select(); err != nil {
		return []interface{}{}, err
	}

	var results = make([]interface{}, len(execs))

	for i := 0; i < len(execs); i++ {
		results[i] = execs[i]
	}

	return results, nil
}

func (exec *Execution) UpdateOne(pool *repository.Pool, ctx context.Context) error {
	conn, err := pool.Acquire()
	if err != nil {
		return err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var execPlaceholder Execution
	execPlaceholder.ID = exec.ID

	err = execPlaceholder.GetOne(pool, ctx, "id = ?", execPlaceholder.ID)

	if err = db.Update(exec); err != nil {
		return err
	}

	return nil
}

func (exec *Execution) DeleteOne(pool *repository.Pool, ctx context.Context) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	r, err := db.Model(exec).Where("id = ?", exec.ID).Delete()
	if err != nil {
		return -1, err
	}

	return r.RowsAffected(), nil
}

func (exec *Execution) SearchToQuery(search [][]string) (string, []string) {
	var searchParams = []string{"id", "job_id", "timeout", "status_code", "created_at"}

	var queries []string
	var query string
	var values []string
	var paginate []string

	if len(search) < 1 || search[0] == nil {
		return query, values
	}

	for i := 0; i < len(searchParams); i++ {
		if search[i][0] == searchParams[i] {
			queries = append(queries, searchParams[i]+" = ?")
			values = append(values, search[i][1])
		}
	}

	if len(queries) > 0 {
		query += " AND " + queries[0]

		for i := 1; i < len(queries); i++ {
			query = queries[i]
		}
	}

	if len(queries) < len(search) {
		for i := 0; i < len(search); i++ {
			if search[i][0] == "offset" {
				paginate = append(paginate, "offset = ?")
			}

			if search[i][0] == "limit" {
				paginate = append(paginate, "limit = ?")
			}
		}
	}

	for i := 0; i < len(paginate); i++ {
		query = paginate[i]
	}

	if len(query) < 1 && len(values) < 1 {
		values = append(values, "null")
		return "id != ?", values
	}

	return query, values
}

func (exec *Execution) ToJson() ([]byte, error) {
	data, err := json.Marshal(exec)
	if err != nil {
		return data, err
	}
	return data, nil
}

func (exec *Execution) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &exec); err != nil {
		return err
	}
	return nil
}
