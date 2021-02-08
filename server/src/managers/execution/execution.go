package execution

import (
	"errors"
	"github.com/go-pg/pg"
	"github.com/victorlenerd/scheduler0/server/src/managers/job"
	"github.com/victorlenerd/scheduler0/server/src/models"
	"github.com/victorlenerd/scheduler0/server/src/utils"
)

type ExecutionManager models.ExecutionModel

func (executionManager *ExecutionManager) CreateOne(pool *utils.Pool) (string, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return "", err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if len(executionManager.JobUUID) < 1 {
		err := errors.New("job uuid is not set")
		return "", err
	}

	jobWithId := job.JobManager{UUID: executionManager.UUID}

	 if getOneJobError := jobWithId.GetOne(pool, executionManager.JobUUID);  err != nil {
		return "", errors.New(getOneJobError.Message)
	}

	executionManager.JobID = jobWithId.ID

	if _, err := db.Model(executionManager).Insert(); err != nil {
		return "", err
	}

	return executionManager.UUID, nil
}

func (executionManager *ExecutionManager) GetOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	countQuery := db.Model(executionManager).Where("uuid = ?", executionManager.UUID)

	count, err := countQuery.Count()
	if count < 1 {
		return 0, nil
	}

	if err != nil {
		return count, err
	}

	selectQuery := db.Model(executionManager).Where("uuid = ?", executionManager.UUID)
	err = selectQuery.Select()

	if err != nil {
		return count, err
	}

	return count, nil
}

func (executionManager *ExecutionManager) GetAll(pool *utils.Pool, jobID string, offset int, limit int, orderBy string) ([]ExecutionManager, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return nil, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	execs := make([]ExecutionManager, 0, limit)

	err = db.
		Model(&execs).
		Where("job_uuid = ?", jobID).
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()

	return execs, nil
}

func (executionManager *ExecutionManager) UpdateOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	execPlaceholder := ExecutionManager{
		UUID: executionManager.UUID,
	}

	_, err = execPlaceholder.GetOne(pool)

	res, err := db.Model(&executionManager).Update(executionManager)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected(), nil
}

func (executionManager *ExecutionManager) DeleteOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	r, err := db.Model(executionManager).Where("id = ?", executionManager.ID).Delete()
	if err != nil {
		return -1, err
	}

	return r.RowsAffected(), nil
}
