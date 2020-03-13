package models

import (
	"context"
	"cron-server/server/migrations"
	"encoding/json"
	"errors"
	"github.com/go-pg/pg"
	"time"
)

type Execution struct {
	ID          string    `json:"id"`
	JobId       string    `json:"job_id"`
	StatusCode  string    `json:"status_code"`
	Timeout     uint64    `json:"timeout"`
	Response    string    `json:"response"`
	Token       string    `json:"token"`
	DateCreated time.Time `json:"date_created"`
}

func (exec *Execution) SetId(id string) {
	exec.ID = id
}

func (exec *Execution) CreateOne(pool *migrations.Pool, ctx context.Context) (string, error) {
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

	if _, err := jobWithId.GetOne(pool, ctx, "id = ?", exec.JobId); err != nil {
		return "", errors.New("job with id does not exist")
	}

	if _, err := db.Model(exec).Insert(); err != nil {
		return "", err
	}

	return exec.ID, nil
}

func (exec *Execution) GetOne(pool *migrations.Pool, ctx context.Context, query string, params interface{}) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	baseQuery := db.Model(&exec).Where(query, params)

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

func (exec *Execution) GetAll(pool *migrations.Pool, ctx context.Context, query string, offset int, limit int, orderBy string, params ...string) (int, []interface{}, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, []interface{}{}, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	ip := make([]interface{}, len(params))

	for i := 0; i < len(params); i++ {
		ip[i] = params[i]
	}

	var execs []Execution

	baseQuery := db.
		Model(&execs).
		Where(query, ip...)

	count, err := baseQuery.Count()
	if err != nil {
		return count, []interface{}{}, err
	}

	err = baseQuery.
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()

	if err != nil {
		return count, []interface{}{}, err
	}

	var results = make([]interface{}, len(execs))

	for i := 0; i < len(execs); i++ {
		results[i] = execs[i]
	}

	return count, results, nil
}

func (exec *Execution) UpdateOne(pool *migrations.Pool, ctx context.Context) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var execPlaceholder Execution
	execPlaceholder.ID = exec.ID

	_, err = execPlaceholder.GetOne(pool, ctx, "id = ?", execPlaceholder.ID)

	res, err := db.Model(&exec).Update(exec)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected(), nil
}

func (exec *Execution) DeleteOne(pool *migrations.Pool, ctx context.Context) (int, error) {
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
	var queries []string
	var query string
	var values []string

	if len(search) < 1 || search[0] == nil {
		return query, values
	}

	for i := 0; i < len(search); i++ {
		if search[i][0] == "created_at" {
			queries = append(queries, "created_at = ?")
			values = append(values, search[i][1])
		}

		if search[i][0] == "id" {
			queries = append(queries, "id = ?")
			values = append(values, search[i][1])
		}

		if search[i][0] == "job_id" {
			queries = append(queries, "job_id = ?")
			values = append(values, search[i][1])
		}

		if search[i][0] == "status_code" {
			queries = append(queries, "status_code = ?")
			values = append(values, search[i][1])
		}

		if search[i][0] == "timeout" {
			queries = append(queries, "timeout = ?")
			values = append(values, search[i][1])
		}
	}

	if len(queries) > 0 {
		query += " OR " + queries[0]

		for i := 1; i < len(queries); i++ {
			query = queries[i]
		}
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
