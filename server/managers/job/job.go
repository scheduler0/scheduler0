package job

import (
	"github.com/go-pg/pg"
	"net/http"
	"scheduler0/server/managers/project"
	"scheduler0/server/models"
	"scheduler0/utils"
)

// Manager job table manager
type Manager models.JobModel

// CreateOne create a new job
func (jobManager *Manager) CreateOne(dbConnection *pg.DB) (string, *utils.GenericError) {
	if len(jobManager.ProjectUUID) < 1 {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "project uuid is not defined")
	}

	if len(jobManager.CallbackUrl) < 1 {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "callback url is required")
	}

	if len(jobManager.Spec) < 1 {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "spec is required")
	}

	projectWithUUID := project.ProjectManager{UUID: jobManager.ProjectUUID}
	e := projectWithUUID.GetOneByUUID(dbConnection)
	if e != nil {
		return "", e
	}

	jobManager.ProjectID = projectWithUUID.ID
	jobManager.ProjectUUID = projectWithUUID.UUID

	if _, err := dbConnection.Model(jobManager).Insert(); err != nil {
		return "", utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	return jobManager.UUID, nil
}

// GetOne returns a single job that matches uuid
func (jobManager *Manager) GetOne(dbConnection *pg.DB, jobUUID string) *utils.GenericError {
	err := dbConnection.Model(jobManager).
		Where("uuid = ?", jobUUID).
		Select()

	if err != nil {
		return utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	return nil
}

// GetAll returns paginated set of jobs that are not archived
func (jobManager *Manager) GetAll(dbConnection *pg.DB, projectUUID string, offset int, limit int, orderBy string) ([]Manager, *utils.GenericError) {
	jobs := make([]Manager, 0, limit)

	err := dbConnection.Model(&jobs).
		Where("project_uuid = ?", projectUUID).
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()


	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	return jobs, nil
}

// UpdateOne updates a job and returns number of affected rows
func (jobManager *Manager) UpdateOne(dbConnection *pg.DB) (int, *utils.GenericError) {
	jobPlaceholder := Manager{
		UUID: jobManager.UUID,
	}

	if jobPlaceholderError := jobPlaceholder.GetOne(dbConnection, jobPlaceholder.UUID); jobPlaceholderError != nil {
		return 0, jobPlaceholderError
	}

	if jobPlaceholder.Spec != jobManager.Spec {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "cannot update cron spec")
	}

	jobManager.ProjectID = jobPlaceholder.ProjectID
	res, err := dbConnection.Model(jobManager).Where("uuid = ? ", jobManager.UUID).Update(jobManager)

	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	return res.RowsAffected(), nil
}

// DeleteOne deletes a job with uuid and returns number of affected row
func (jobManager *Manager) DeleteOne(dbConnection *pg.DB) (int, *utils.GenericError) {
	if r, err := dbConnection.Model(jobManager).Where("uuid = ?", jobManager.UUID).Delete(); err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	} else {
		return r.RowsAffected(), nil
	}
}

// GetJobsTotalCount returns total number of jobs
func (jobManager *Manager) GetJobsTotalCount(dbConnection *pg.DB) (int, *utils.GenericError) {
	if count, err := dbConnection.Model(jobManager).Count(); err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	} else {
		return count, nil
	}
}

// GetJobsTotalCountByProjectUUID returns the number of jobs for project with uuid
func (jobManager *Manager) GetJobsTotalCountByProjectUUID(dbConnection *pg.DB, projectUUID string) (int, *utils.GenericError) {
	if count, err := dbConnection.Model(jobManager).Where("project_uuid = ?", projectUUID).Count(); err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	} else {
		return count, nil
	}
}

// GetJobsPaginated returns a set of jobs starting at offset with the limit
func (jobManager *Manager) GetJobsPaginated(dbConnection *pg.DB, projectUUID string, offset int, limit int) ([]Manager, int, *utils.GenericError) {
	total, err := jobManager.GetJobsTotalCountByProjectUUID(dbConnection, projectUUID)

	if err != nil {
		return nil, 0, err
	}

	jobManagers, err := jobManager.GetAll(dbConnection, projectUUID, offset, limit, "date_created")
	if err != nil {
		return nil, total, err
	}

	return jobManagers, total, nil
}
