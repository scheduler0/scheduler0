package models

import (
	"cron-server/server/misc"
	"encoding/json"
	"github.com/go-pg/pg"
	"github.com/segmentio/ksuid"
	"reflect"
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
	State            State     `json:"state,omitempty"`
	StartDate        time.Time `json:"total_execs,omitempty"`
	EndDate          time.Time `json:"end_date,omitempty"`
	NextTime         time.Time `json:"next_time,omitempty"`
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

	jd.ID = ksuid.New().String()

	_, err := db.Model(jd).Insert()
	if err != nil {
		return "", err
	}

	return jd.ID, nil
}

func (jd *Job) GetOne() error {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	err := db.Model(jd).Where("id = ?", jd.ID).Select()
	if err != nil {
		return err
	}

	return nil
}

func (jd *Job) GetAll() (interface{}, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	var jobs []Job

	err := db.Model(&jobs).Where("project_id =", jd.ID).Select()
	if err != nil {
		return jobs, err
	}

	vp := reflect.New(reflect.TypeOf(jobs))
	vp.Elem().Set(reflect.ValueOf(jobs))
	return vp.Interface(), nil
}

func (jd *Job) UpdateOne() error {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	err := db.Update(jd)
	if err != nil {
		return err
	}

	return nil
}

func (jd *Job) DeleteOne() error {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	_, err := db.Model(jd).Where("id = ?", jd.ID).Delete()
	if err != nil {
		return err
	}

	return nil
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
