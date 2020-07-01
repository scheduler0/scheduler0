package managers

import (
	"cron-server/server/src/misc"
	"cron-server/server/src/models"
	"errors"
	"github.com/go-pg/pg"
	"github.com/robfig/cron"
	"github.com/segmentio/ksuid"
	"log"
	"time"
)

type JobManager models.JobModel

func (jd *JobManager) CreateOne(pool *misc.Pool) (string, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return "", err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if len(jd.ProjectId) < 1 {
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

	projectWithId := ProjectManager{ID: jd.ProjectId}
	c, _ := projectWithId.GetOne(pool, "id = ?", jd.ProjectId)
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
	jd.DateCreated = time.Now().UTC()
	jd.StartDate = jd.StartDate.UTC()
	jd.ID = ksuid.New().String()

	if _, err := db.Model(jd).Insert(); err != nil {
		return "", err
	}

	return jd.ID, nil
}

func (jd *JobManager) GetOne(pool *misc.Pool, query string, params interface{}) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	baseQuery := db.Model(jd).Where(query, params)
	count, err := baseQuery.Count()
	if count < 1 {
		return 0, nil
	}

	err = baseQuery.Select()

	if err != nil {
		return 0, err
	}

	return count, nil
}

func (jd *JobManager) GetAll(pool *misc.Pool, query string, offset int, limit int, orderBy string, params ...string) (int, []interface{}, error) {
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

	var jobs []JobManager

	baseQuery := db.Model(&jobs).Where(query, ip...)

	count, err := baseQuery.Count()
	if err != nil {
		return 0, []interface{}{}, err
	}

	err = baseQuery.
		Order(orderBy).
		Offset(offset).
		Limit(limit).
		Select()

	if err != nil {
		return 0, []interface{}{}, err
	}

	var results = make([]interface{}, len(jobs))

	for i := 0; i < len(jobs); i++ {
		results[i] = jobs[i]
	}

	log.Println("results--", results, count)

	return count, results, nil
}

func (jd *JobManager) UpdateOne(pool *misc.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var jobPlaceholder JobManager
	jobPlaceholder.ID = jd.ID

	_, err = jobPlaceholder.GetOne(pool, "id = ?", jobPlaceholder.ID)

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

func (jd *JobManager) DeleteOne(pool *misc.Pool) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return -1, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if r, err := db.Model(jd).Where("id = ?", jd.ID).Delete(); err != nil {
		return -1, err
	} else {
		return r.RowsAffected(), nil
	}
}
