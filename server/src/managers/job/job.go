package job

import (
	"github.com/go-pg/pg"
	"github.com/robfig/cron"
	"net/http"
	"scheduler0/server/src/managers/project"
	"scheduler0/server/src/models"
	"scheduler0/server/src/utils"
	"time"
)

type JobManager models.JobModel

func (jobManager *JobManager) CreateOne(pool *utils.Pool) (string, *utils.GenericError) {
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

	schedule, err := cron.ParseStandard(jobManager.CronSpec)
	if err != nil {
		return "", utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	jobManager.ProjectID = projectWithUUID.ID
	jobManager.ProjectUUID = projectWithUUID.UUID
	jobManager.State = models.InActiveJob
	jobManager.NextTime = schedule.Next(jobManager.StartDate)
	jobManager.TotalExecs = -1
	jobManager.StartDate = jobManager.StartDate.UTC()

	if _, err := db.Model(jobManager).Insert(); err != nil {
		return "", utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	return jobManager.UUID, nil
}

func (jobManager *JobManager) GetOne(pool *utils.Pool, jobUUID string) *utils.GenericError {
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

func (jobManager *JobManager) GetAll(pool *utils.Pool, projectUUID string, offset int, limit int, orderBy string) ([]JobManager, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	jobs := make([]JobManager, 0, limit)

	err = db.Model(&jobs).
		Where("project_uuid = ?", projectUUID).
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()

	return jobs, nil
}

func (jobManager *JobManager) UpdateOne(pool *utils.Pool) (int, *utils.GenericError) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, utils.HTTPGenericError(http.StatusBadRequest, err.Error())
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	jobPlaceholder := JobManager{
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

func (jobManager *JobManager) DeleteOne(pool *utils.Pool) (int, *utils.GenericError) {
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

func (jobManager *JobManager) GetJobsTotalCount(pool *utils.Pool) (int, *utils.GenericError) {
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

func (jobManager *JobManager) GetJobsTotalCountByProjectID(pool *utils.Pool, projectUUID string) (int, *utils.GenericError) {
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

func (jobManager *JobManager) GetJobsPaginated(pool *utils.Pool, projectUUID string, offset int, limit int) ([]JobManager, int, *utils.GenericError) {
	total, err := jobManager.GetJobsTotalCountByProjectID(pool, projectUUID)

	if err != nil {
		return nil, 0, err
	}

	jobManagers, err := jobManager.GetAll(pool, projectUUID, offset, limit, "date_created")
	if err != nil {
		return nil, total, err
	}

	return jobManagers, total, nil
}
