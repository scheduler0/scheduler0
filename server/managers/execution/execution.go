package execution

import (
	"github.com/go-pg/pg"
	"net/http"
	"scheduler0/server/managers/job"
	"scheduler0/server/models"
	"scheduler0/utils"
)

// Manager this manager handles interacting with the database for all execution entity type
type Manager models.ExecutionModel

// CreateOne creates a new executions
func (executionManager *Manager) CreateOne(dbConnection *pg.DB) (string, *utils.GenericError) {
	if len(executionManager.JobUUID) < 1 {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "job uuid is not set")
	}

	jobWithID := job.Manager{UUID: executionManager.UUID}

	if getOneJobError := jobWithID.GetOne(dbConnection, executionManager.JobUUID); getOneJobError != nil {
		return "", getOneJobError
	}

	executionManager.JobID = jobWithID.ID

	if _, err := dbConnection.Model(executionManager).Insert(); err != nil {
		return "", utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return executionManager.UUID, nil
}

// GetOne returns a executions with the UUID
func (executionManager *Manager) GetOne(dbConnection *pg.DB) (int, *utils.GenericError) {
	countQuery := dbConnection.Model(executionManager).Where("uuid = ?", executionManager.UUID)

	count, err := countQuery.Count()
	if count < 1 {
		return 0, nil
	}

	if err != nil {
		return count, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	selectQuery := dbConnection.Model(executionManager).Where("uuid = ?", executionManager.UUID)
	err = selectQuery.Select()

	if err != nil {
		return count, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return count, nil
}

// List returns a list of executions
func (executionManager *Manager) List(dbConnection *pg.DB, jobID string, offset int, limit int, orderBy string) ([]Manager, *utils.GenericError) {
	execs := make([]Manager, 0, limit)

	err := dbConnection.
		Model(&execs).
		Where("job_uuid = ?", jobID).
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()

	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return execs, nil
}

// Count returns the total number of executions with JobID matching jobID
func (executionManager *Manager) Count(dbConnection *pg.DB, jobID string) (int, *utils.GenericError) {
	count, err := dbConnection.
		Model(executionManager).
		Where("job_uuid = ?", jobID).
		Count()

	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return count, nil
}

// UpdateOne updates a single execution entity
func (executionManager *Manager) UpdateOne(dbConnection *pg.DB) (int, *utils.GenericError) {
	executionManagerPlaceholder := Manager{
		ID: executionManager.ID,
		UUID: executionManager.UUID,
	}

	_, errorGettingOneManager := executionManagerPlaceholder.GetOne(dbConnection)
	if errorGettingOneManager != nil {
		return 0, errorGettingOneManager
	}

	res, err := dbConnection.Model(executionManager).
		Where("id  = ? ", executionManager.ID).
		Update(executionManager)
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return res.RowsAffected(), nil
}

// DeleteOne deletes a single execution entity from the database
func (executionManager *Manager) DeleteOne(dbConnection *pg.DB) (int, *utils.GenericError) {
	r, err := dbConnection.Model(executionManager).
		Where("id = ?", executionManager.ID).
		Delete()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return r.RowsAffected(), nil
}

// FindJobExecutionPlaceholderByUUID returns an execution placeholder for a job that's not been executed
func (executionManager *Manager) FindJobExecutionPlaceholderByUUID(dbConnection *pg.DB, jobUUID string) (int, *utils.GenericError, []Manager) {
	countQuery := dbConnection.Model(executionManager).Where("job_uuid = ? AND time_executed is NULL", jobUUID)

	count, countError := countQuery.Count()
	if countError != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, countError.Error()), nil
	}
	if count < 1 {
		return 0, nil, nil
	}

	executionManagers := make([]Manager, 0, count)

	getError := dbConnection.
		Model(&executionManagers).
		Where("job_uuid = ? AND time_executed is NULL", jobUUID).
		Order("time_added DESC").
		Select()

	if getError != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, getError.Error()), nil
	}

	return count, nil, executionManagers
}