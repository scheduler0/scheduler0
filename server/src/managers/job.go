package managers

import (
	"cron-server/server/src/models"
	"cron-server/server/src/utils"
	"errors"
	"github.com/go-pg/pg"
	"github.com/robfig/cron"
	"github.com/segmentio/ksuid"
	"time"
)

type JobManager models.JobModel

func (jd *JobManager) CreateOne(pool *utils.Pool) (string, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return "", err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if len(jd.ProjectID) < 1 {
		err := errors.New("project id is not sets")
		return "", err
	}

	if jd.StartDate.IsZero() {
		err := errors.New("start date cannot be zero")
		return "", err
	}

	if jd.StartDate.UTC().Before(time.Now().UTC()) {
		err := errors.New("start date cannot be in the past")
		return "", err
	}

	if !jd.EndDate.IsZero() && jd.EndDate.UTC().Before(jd.StartDate.UTC()) {
		err := errors.New("end date cannot be in the past")
		return "", err
	}

	if len(jd.CallbackUrl) < 1 {
		err := errors.New("callback url is required")
		return "", err
	}

	if len(jd.CronSpec) < 1 {
		err := errors.New("cron spec is required")
		return "", err
	}

	projectWithId := ProjectManager{ID: jd.ProjectID}
	c, _ := projectWithId.GetOne(pool)
	if c < 1 {
		return "", errors.New("project with id does not exist")
	}

	schedule, err := cron.ParseStandard(jd.CronSpec)
	if err != nil {
		return "", err
	}

	jd.State = models.InActiveJob
	jd.NextTime = schedule.Next(jd.StartDate)
	jd.TotalExecs = -1
	jd.StartDate = jd.StartDate.UTC()
	jd.ID = ksuid.New().String()

	if _, err := db.Model(jd).Insert(); err != nil {
		return "", err
	}

	return jd.ID, nil
}

func (jd *JobManager) GetOne(pool *utils.Pool, jobID string) error {
	conn, err := pool.Acquire()
	if err != nil {
		return err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	err = db.Model(jd).
		Where("id = ?", jobID).
		Select()

	if err != nil {
		return err
	}

	return nil
}

func (jd *JobManager) GetAll(pool *utils.Pool, projectID string, offset int, limit int, orderBy string) ([]JobManager, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return nil, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	jobs := make([]JobManager, 0, limit)

	err = db.Model(&jobs).
		Where("project_id = ?", projectID).
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()

	return jobs, nil
}

func (jd *JobManager) UpdateOne(pool *utils.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var jobPlaceholder JobManager
	jobPlaceholder.ID = jd.ID

	err = jobPlaceholder.GetOne(pool, jobPlaceholder.ID)
	if err != nil {
		return 0, err
	}

	if jobPlaceholder.CronSpec != jd.CronSpec {
		return 0, errors.New("cannot update cron spec")
	}

	if !jd.EndDate.IsZero() && jd.EndDate.UTC().Before(jobPlaceholder.StartDate.UTC()) {
		err := errors.New("end date cannot be in the past")
		return 0, err
	}

	if !jd.EndDate.IsZero() && len(jd.EndDate.String()) < 1 {
		err := errors.New("end date cannot be in the past")
		return 0, err
	}

	res, err := db.Model(jd).Where("id = ? ", jd.ID).Update(jd)

	if err != nil {
		return 0, err
	}

	return res.RowsAffected(), nil
}

func (jd *JobManager) DeleteOne(pool *utils.Pool, jobID string) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if r, err := db.Model(jd).Where("id = ?", jobID).Delete(); err != nil {
		return -1, err
	} else {
		return r.RowsAffected(), nil
	}
}
