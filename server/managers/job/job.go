package job

import (
	"github.com/go-pg/pg"
	"net/http"
	"scheduler0/server/managers/project"
	"scheduler0/server/models"
	"scheduler0/utils"
	"time"
)

type Manager models.JobModel

// CreateOne create a new job
func (jobManager *Manager) CreateOne(pool *utils.Pool) (string, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return "", utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if len(jobManager.ProjectUUID) < 1 {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "project id is not sets")
	}

	if jobManager.StartDate.IsZero() {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "start date cannot be zero")
	}

	if jobManager.StartDate.UTC().Before(time.Now().UTC()) {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "start date cannot be in the past: "+jobManager.StartDate.String()+", now "+time.Now().UTC().String())
	}

	if !jobManager.EndDate.IsZero() && jobManager.EndDate.UTC().Before(jobManager.StartDate.UTC()) {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "end date cannot be in the past")
	}

	if len(jobManager.CallbackUrl) < 1 {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "callback url is required")
	}

	if len(jobManager.CronSpec) < 1 {
		return "", utils.HTTPGenericError(http.StatusBadRequest, "cron spec is required")
	}

	projectWithUUID := project.ProjectManager{UUID: jobManager.ProjectUUID}
	e := projectWithUUID.GetOneByUUID(pool)
	if e != nil {
		return "", e
	}


	jobManager.ProjectID = projectWithUUID.ID
	jobManager.ProjectUUID = projectWithUUID.UUID
	jobManager.StartDate = jobManager.StartDate.UTC()

	if _, err := db.Model(jobManager).Insert(); err != nil {
		return "", utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	return jobManager.UUID, nil
}

// GetOne returns a single job that matches uuid
func (jobManager *Manager) GetOne(pool *utils.Pool, jobUUID string) *utils.GenericError {
	conn, err := pool.Acquire()
	if err != nil {
		return utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	err = db.Model(jobManager).
		Where("uuid = ?", jobUUID).
		Select()

	if err != nil {
		return utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	return nil
}

// GetAll returns paginated set of jobs that are not archived
func (jobManager *Manager) GetAll(pool *utils.Pool, projectUUID string, offset int, limit int, orderBy string) ([]Manager, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	jobs := make([]Manager, 0, limit)

	err = db.Model(&jobs).
		Where("project_uuid = ? AND archived = ?", projectUUID, false).
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()

	return jobs, nil
}

// UpdateOne updates a job and returns number of affected rows
func (jobManager *Manager) UpdateOne(pool *utils.Pool) (int, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	jobPlaceholder := Manager{
		UUID: jobManager.UUID,
	}

	if jobPlaceholderError := jobPlaceholder.GetOne(pool, jobPlaceholder.UUID); jobPlaceholderError != nil {
		return 0, jobPlaceholderError
	}

	if jobPlaceholder.CronSpec != jobManager.CronSpec {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "cannot update cron spec")
	}

	if !jobManager.EndDate.IsZero() && jobManager.EndDate.UTC().Before(jobPlaceholder.StartDate.UTC()) {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "end date cannot be in the past")
	}

	if !jobManager.EndDate.IsZero() && len(jobManager.EndDate.String()) < 1 {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, "end date cannot be in the past")
	}

	jobManager.ProjectID = jobPlaceholder.ProjectID
	res, err := db.Model(jobManager).Where("uuid = ? ", jobManager.UUID).Update(jobManager)

	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	return res.RowsAffected(), nil
}

// DeleteOne deletes a job with uuid and returns number of affected row
func (jobManager *Manager) DeleteOne(pool *utils.Pool) (int, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if r, err := db.Model(jobManager).Where("uuid = ?", jobManager.UUID).Delete(); err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	} else {
		return r.RowsAffected(), nil
	}
}

// GetJobsTotalCount returns total number of jobs
func (jobManager *Manager) GetJobsTotalCount(pool *utils.Pool) (int, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if count, err := db.Model(jobManager).Count(); err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	} else {
		return count, nil
	}
}

// GetJobsTotalCountByProjectUUID returns the number of jobs for project with uuid
func (jobManager *Manager) GetJobsTotalCountByProjectUUID(pool *utils.Pool, projectUUID string) (int, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if count, err := db.Model(jobManager).Where("project_uuid = ?", projectUUID).Count(); err != nil {
		return -1, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	} else {
		return count, nil
	}
}

// GetJobsPaginated returns a set of jobs starting at offset with the limit
func (jobManager *Manager) GetJobsPaginated(pool *utils.Pool, projectUUID string, offset int, limit int) ([]Manager, int, *utils.GenericError) {
	total, err := jobManager.GetJobsTotalCountByProjectUUID(pool, projectUUID)

	if err != nil {
		return nil, 0, err
	}

	jobManagers, err := jobManager.GetAll(pool, projectUUID, offset, limit, "date_created")
	if err != nil {
		return nil, total, err
	}

	return jobManagers, total, nil
}
