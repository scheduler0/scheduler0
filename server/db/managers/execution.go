package managers

import (
	"context"
	"cron-server/server/db"
	"errors"
	"github.com/go-pg/pg"
	"time"
)

type ExecutionManager struct {
	ID          string    `json:"id"`
	JobId       string    `json:"job_id"`
	StatusCode  string    `json:"status_code"`
	Timeout     uint64    `json:"timeout"`
	Response    string    `json:"response"`
	Token       string    `json:"token"`
	DateCreated time.Time `json:"date_created"`
}

func (exec *ExecutionManager) CreateOne(pool *db.Pool, ctx context.Context) (string, error) {
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

	jobWithId := JobManager{ID: exec.JobId}

	if _, err := jobWithId.GetOne(pool, ctx, "id = ?", exec.JobId); err != nil {
		return "", errors.New("job with id does not exist")
	}

	if _, err := db.Model(exec).Insert(); err != nil {
		return "", err
	}

	return exec.ID, nil
}

func (exec *ExecutionManager) GetOne(pool *db.Pool, ctx context.Context, query string, params interface{}) (int, error) {
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

func (exec *ExecutionManager) GetAll(pool *db.Pool, ctx context.Context, query string, offset int, limit int, orderBy string, params ...string) (int, []interface{}, error) {
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

	var execs []ExecutionManager

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

func (exec *ExecutionManager) UpdateOne(pool *db.Pool, ctx context.Context) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var execPlaceholder ExecutionManager
	execPlaceholder.ID = exec.ID

	_, err = execPlaceholder.GetOne(pool, ctx, "id = ?", execPlaceholder.ID)

	res, err := db.Model(&exec).Update(exec)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected(), nil
}

func (exec *ExecutionManager) DeleteOne(pool *db.Pool, ctx context.Context) (int, error) {
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