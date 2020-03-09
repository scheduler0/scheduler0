package models

import (
	"context"
	"cron-server/server/misc"
	"cron-server/server/migrations"
	"encoding/json"
	"errors"
	"github.com/go-pg/pg"
	"github.com/robfig/cron"
	"github.com/segmentio/ksuid"
	"log"
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
	ID             string    `json:"id,omitempty" pg:",notnull"`
	ProjectId      string    `json:"project_id" pg:",notnull"`
	Description    string    `json:"description" pg:",notnull"`
	CronSpec       string    `json:"cron_spec,omitempty" pg:",notnull"`
	TotalExecs     int64     `json:"total_execs,omitempty" pg:",notnull"`
	Data           string    `json:"data,omitempty"`
	CallbackUrl    string    `json:"callback_url" pg:",notnull"`
	LastStatusCode int       `json:"last_status_code"`
	Timezone       string    `json:"timezone"`
	State          State     `json:"state,omitempty" pg:",notnull"`
	StartDate      time.Time `json:"start_date,omitempty" pg:",notnull"`
	EndDate        time.Time `json:"end_date,omitempty"`
	NextTime       time.Time `json:"next_time,omitempty" pg:",notnull"`
	DateCreated    time.Time `json:"date_created" pg:",notnull"`
}

var psgc = misc.GetPostgresCredentials()

func (jd *Job) SetId(id string) {
	jd.ID = id
}

func (jd *Job) CreateOne(pool *migrations.Pool, ctx context.Context) (string, error) {
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

	projectWithId := Project{ID: jd.ProjectId}
	c, _ := projectWithId.GetOne(pool, ctx, "id = ?", jd.ProjectId)
	if c < 1 {
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
	jd.ID = ksuid.New().String()

	if _, err := db.Model(jd).Insert(); err != nil {
		return "", err
	}

	return jd.ID, nil
}

func (jd *Job) GetOne(pool *migrations.Pool, ctx context.Context, query string, params interface{}) (int, error) {
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

func (jd *Job) GetAll(pool *migrations.Pool, ctx context.Context, query string, offset int, limit int, orderBy string, params ...string) (int, []interface{}, error) {
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

	var jobs []Job

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

func (jd *Job) UpdateOne(pool *migrations.Pool, ctx context.Context) (int, error) {
	conn, err := pool.Acquire()
	if err != nil {
		return 0, err
	}
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var jobPlaceholder Job
	jobPlaceholder.ID = jd.ID

	_, err = jobPlaceholder.GetOne(pool, ctx, "id = ?", jobPlaceholder.ID)

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

func (jd *Job) DeleteOne(pool *migrations.Pool, ctx context.Context) (int, error) {
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

	if len(query) < 1 && len(values) < 1 {
		values = append(values, "null")
		return "id != ?", values
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
