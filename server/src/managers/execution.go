package managers

import (
	"cron-server/server/src/utils"
	"cron-server/server/src/models"
	"errors"
	"github.com/go-pg/pg"
	"github.com/segmentio/ksuid"
)

type ExecutionManager models.ExecutionModel

func (exec *ExecutionManager) CreateOne(pool *utils.Pool) (string, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return "", err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if len(exec.JobID) < 1 {
		err := errors.New("job id is not set")
		return "", err
	}

	jobWithId := JobManager{ID: exec.JobID}

	if count, _ := jobWithId.GetOne(pool, "id = ?", exec.JobID); count < 1 {
		return "", errors.New("job with id does not exist")
	}

	exec.ID = ksuid.New().String()

	if _, err := db.Model(exec).Insert(); err != nil {
		return "", err
	}

	return exec.ID, nil
}

func (exec *ExecutionManager) GetOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	countQuery := db.Model(exec).Where("id = ?", exec.ID)

	count, err := countQuery.Count()
	if count < 1 {
		return 0, nil
	}

	if err != nil {
		return count, err
	}

	selectQuery := db.Model(exec).Where("id = ?", exec.ID)
	err = selectQuery.Select()

	if err != nil {
		return count, err
	}

	return count, nil
}

func (exec *ExecutionManager) GetAll(pool *utils.Pool, jobID string, offset int, limit int, orderBy string) ([]ExecutionManager, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return nil, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	execs := make([]ExecutionManager, 0, limit)

	err = db.
		Model(&execs).
		Where("job_id = ?", jobID).
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()

	return execs, nil
}

func (exec *ExecutionManager) UpdateOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var execPlaceholder ExecutionManager
	execPlaceholder.ID = exec.ID

	_, err = execPlaceholder.GetOne(pool)

	res, err := db.Model(&exec).Update(exec)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected(), nil
}

func (exec *ExecutionManager) DeleteOne(pool *utils.Pool) (int, error) {
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
