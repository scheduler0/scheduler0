package models

import (
	"context"
	"cron-server/server/misc"
	"cron-server/server/repository"
	"encoding/json"
	"errors"
	"github.com/go-pg/pg"
	"github.com/robfig/cron"
	"github.com/segmentio/ksuid"
	"time"
)

/*
	State for a job could be:

	- InActive: 1
	- Active:   2
	- Stale:   -1
*/
type State int

const (
	InActiveJob State = iota + 1
	ActiveJob
	StaleJob
)

// Job domain internal representation of job
type Job struct {
	ID               string    `json:"id,omitempty"`
	ProjectId        string    `json:"project_id"`
	Description      string    `json:"description"`
	CronSpec         string    `json:"cron_spec,omitempty"`
	TotalExecs       int64     `json:"total_execs,omitempty"`
	MissedExecs      int64     `json:"missed_execs"`
	SecsBetweenExecs float64   `json:"secs_between_execs,omitempty"`
	Data             string    `json:"data,omitempty"`
	CallbackUrl      string    `json:"callback_url"`
	LastStatusCode   int       `json:"last_status_code"`
	State            State     `json:"state,omitempty"`
	StartDate        time.Time `json:"start_date,omitempty"`
	EndDate          time.Time `json:"end_date,omitempty"`
	NextTime         time.Time `json:"next_time,omitempty"`
	DateCreated      time.Time `json:"date_created"`
}


var psgc = misc.GetPostgresCredentials()

func (jd *Job) SetId(id string) {
	jd.ID = id
}

func (jd *Job) CreateOne(pool *repository.Pool, ctx context.Context) (string, error) {
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

	if jd.StartDate.Before(time.Now().UTC()) {
		err := errors.New("start date cannot be in the past")
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

	projectWithId := Project{ID: jd.ProjectId}

	if err := projectWithId.GetOne(pool, ctx, "id = ?", jd.ProjectId); err != nil {
		return "", errors.New("project with id does not exist")
	}

	schedule, err := cron.ParseStandard(jd.CronSpec)
	if err != nil {
		return "", err
	}

	jd.State = InActiveJob
	jd.NextTime = schedule.Next(jd.StartDate)
	jd.TotalExecs = -1
	jd.DateCreated = time.Now().UTC()
	jd.StartDate = jd.StartDate.UTC()
	jd.SecsBetweenExecs = jd.NextTime.UTC().Sub(jd.StartDate.UTC()).Seconds()
	jd.ID = ksuid.New().String()

	if _, err := db.Model(jd).Insert(); err != nil {
		return "", err
	}

	return jd.ID, nil
}

func (jd *Job) GetOne(pool *repository.Pool, ctx context.Context, query string, params interface{}) error {
	conn, err := pool.Acquire()
	if err != nil {
		return err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	if err := db.Model(jd).Where(query, params).Select(); err != nil {
		return err
	}

	return nil
}

func (jd *Job) GetAll(pool *repository.Pool, ctx context.Context, query string, params ...string) ([]interface{}, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return []interface{}{}, err
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	ip := make([]interface{}, len(params))

	for i := 0; i < len(params); i++ {
		ip[i] = params[i]
	}

	var jobs []Job

	if err := db.Model(&jobs).Where(query, ip...).Select(); err != nil {
		return []interface{}{}, err
	}

	var results = make([]interface{}, len(jobs))

	for i := 0; i < len(jobs); i++ {
		results[i] = jobs[i]
	}

	return results, nil
}

func (jd *Job) UpdateOne(pool *repository.Pool, ctx context.Context) error {
	conn, err := pool.Acquire()
	if err != nil {
		return err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var jobPlaceholder Job
	jobPlaceholder.ID = jd.ID

	err = jobPlaceholder.GetOne(pool, ctx, "id = ?", jobPlaceholder.ID)

	if jobPlaceholder.CronSpec != jd.CronSpec {
		return errors.New("cannot update cron spec")
	}

	if err = db.Update(jd); err != nil {
		return err
	}

	return nil
}

func (jd *Job) DeleteOne(pool *repository.Pool, ctx context.Context) (int, error) {
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

func (jd *Job) SearchToQuery(search [][]string) (string, []string) {
	var queries []string
	var query string
	var values []string

	if len(search) < 1 || search[0] == nil {
		return query, values
	}

	for i := 0; i < len(search); i++ {
		if search[i][0] == "id" {
			queries = append(queries, "id = ?")
			values = append(values, search[i][1])
		}

		if search[i][0] == "project_id" {
			queries = append(queries, "project_id = ?")
			values = append(values, search[i][1])
		}

		if search[i][0] == "description" {
			queries = append(queries, "description LIKE ?")
			values = append(values, "%"+search[i][1]+"%")
		}
	}

	for i := 0; i < len(queries); i++ {
		if i != 0 {
			query += " AND " + queries[i]
		} else {
			query = queries[i]
		}
	}

	return query, values
}

func (jd *Job) ToJson() ([]byte, error) {
	if data, err := json.Marshal(jd); err != nil {
		return data, err
	} else {
		return data, nil
	}
}

func (jd *Job) FromJson(body []byte) error {
	if err := json.Unmarshal(body, &jd); err != nil {
		return err
	}
	return nil
}
