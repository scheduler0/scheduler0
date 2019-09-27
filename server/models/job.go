package models

import (
	"cron-server/server/misc"
	"encoding/json"
	"errors"
	"github.com/go-pg/pg"
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
	CronSpec         string    `json:"cron_spec,omitempty"`
	TotalExecs       int64     `json:"total_execs,omitempty"`
	MissedExecs      int64     `json:"missed_execs"`
	SecsBetweenExecs float64   `json:"secs_between_execs,omitempty"`
	Data             string    `json:"data,omitempty"`
	CallbackUrl      string    `json:"callback_url"`
	LastStatusCode   int       `json:"last_status_code"`
	State            State     `json:"state,omitempty"`
	StartDate        time.Time `json:"total_execs,omitempty"`
	EndDate          time.Time `json:"end_date,omitempty"`
	NextTime         time.Time `json:"next_time,omitempty"`
}

type InboundJob struct {
	ID          string    `json:"id,omitempty"`
	ProjectId   string    `json:"project_id"`
	CronSpec    string    `json:"cron_spec,omitempty"`
	Data        string    `json:"data,omitempty"`
	CallbackUrl string    `json:"callback_url"`
	State       State     `json:"state,omitempty"`
	StartDate   time.Time `json:"total_execs,omitempty"`
	EndDate     time.Time `json:"end_date,omitempty"`
}

func (i *InboundJob) ToModel() Job {
	return Job{
		ID:          i.ID,
		ProjectId:   i.ProjectId,
		CronSpec:    i.CronSpec,
		Data:        i.Data,
		CallbackUrl: i.CallbackUrl,
		State:       i.State,
		StartDate:   i.StartDate,
		EndDate:     i.EndDate,
	}
}

var psgc = misc.GetPostgresCredentials()

func (jd *Job) SetId(id string) {
	jd.ID = id
}

func (jd *Job) CreateOne() (string, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	if len(jd.ProjectId) < 1 {
		err := errors.New("project id is not sets")
		return "", err
	}

	if jd.StartDate.IsZero() {
		err := errors.New("start date cannot be zero")
		return "", err
	}

	if len(jd.CronSpec) < 1 {
		err := errors.New("cron spec is required")
		return "", err
	}

	projectWithId := Project{ID: jd.ProjectId}

	err := projectWithId.GetOne("id = ?", jd.ProjectId)
	if err != nil {
		return "", err
	}

	jd.ID = ksuid.New().String()

	_, err = db.Model(jd).Insert()
	if err != nil {
		return "", err
	}

	return jd.ID, nil
}

func (jd *Job) GetOne(query string, params interface{}) error {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	err := db.Model(jd).Where(query, params).Select()
	if err != nil {
		return err
	}

	return nil
}

func (jd *Job) GetAll(query string, params interface{}) ([]interface{}, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	var jobs []Job

	err := db.Model(&jobs).Where(query, params).Select()
	if err != nil {
		return []interface{}{}, err
	}

	var results = make([]interface{}, len(jobs))

	for i := 0; i < len(jobs); i++ {
		results[i] = jobs[i]
	}

	return results, nil
}

func (jd *Job) UpdateOne() error {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	var jobPlaceholder Job
	jobPlaceholder.ID = jd.ID

	err := jobPlaceholder.GetOne("id = ?", jobPlaceholder.ID)

	if jobPlaceholder.CronSpec != jd.CronSpec {
		return errors.New("cannot update cron spec")
	}

	err = db.Update(jd)
	if err != nil {
		return err
	}

	return nil
}

func (jd *Job) DeleteOne() (int, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	r, err := db.Model(jd).Where("id = ?", jd.ID).Delete()
	if err != nil {
		return -1, err
	}

	return r.RowsAffected(), nil
}

func (jd *Job) ToJson() []byte {
	data, err := json.Marshal(jd)

	if err != nil {
		panic(err)
	}

	return data
}

func (jd *Job) FromJson(body []byte) {
	err := json.Unmarshal(body, &jd)

	if err != nil {
		panic(err)
	}
}
