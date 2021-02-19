package execution

import (
	"github.com/go-pg/pg"
	"net/http"
	"scheduler0/server/src/managers/job"
	"scheduler0/server/src/models"
	"scheduler0/server/src/utils"
)

// Manager this manager handles interacting with the database for all execution entity type
type Manager models.ExecutionModel

// CreateOne creates a new executions
func (executionManager *Manager) CreateOne(pool *utils.Pool) (string, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return "", utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if len(executionManager.JobUUID) < 1 {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "job uuid is not set")
	}

	jobWithID := job.JobManager{UUID: executionManager.UUID}

	if getOneJobError := jobWithID.GetOne(pool, executionManager.JobUUID); err != nil {
		return "", getOneJobError
	}

	executionManager.JobID = jobWithID.ID

	if _, err := db.Model(executionManager).Insert(); err != nil {
		return "", utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return executionManager.UUID, nil
}

// GetOne returns a executions with the UUID
func (executionManager *Manager) GetOne(pool *utils.Pool) (int, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	countQuery := db.Model(executionManager).Where("uuid = ?", executionManager.UUID)

	count, err := countQuery.Count()
	if count < 1 {
		return 0, nil
	}

	if err != nil {
		return count, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	selectQuery := db.Model(executionManager).Where("uuid = ?", executionManager.UUID)
	err = selectQuery.Select()

	if err != nil {
		return count, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return count, nil
}

// List returns a list of executions
func (executionManager *Manager) List(pool *utils.Pool, jobID string, offset int, limit int, orderBy string) ([]Manager, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	execs := make([]Manager, 0, limit)

	err = db.
		Model(&execs).
		Where("job_uuid = ?", jobID).
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()

	return execs, nil
}

// Count returns the total number of executions with JobID matching jobID
func (executionManager *Manager) Count(pool *utils.Pool, jobID string) (int, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	count, err := db.
		Model(executionManager).
		Where("job_uuid = ?", jobID).
		Count()

	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return count, nil
}

// UpdateOne updates a single execution entity
func (executionManager *Manager) UpdateOne(pool *utils.Pool) (int, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	executionManagerPlaceholder := Manager{
		UUID: executionManager.UUID,
	}

	_, errorGettingOneManager := executionManagerPlaceholder.GetOne(pool)
	if errorGettingOneManager != nil {
		return 0, errorGettingOneManager
	}

	res, err := db.Model(&executionManager).Update(executionManager)
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return res.RowsAffected(), nil
}


// DeleteOne deletes a single execution entity from the database
func (executionManager *Manager) DeleteOne(pool *utils.Pool) (int, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	r, err := db.Model(executionManager).
		Where("id = ?", executionManager.ID).
		Delete()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return r.RowsAffected(), nil
}
