// Database layer

package repo

import (
	"cron-server/job"
	"cron-server/misc"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"github.com/segmentio/ksuid"
)

var psgc = misc.GetPostgresCredentials()

func Setup() {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	_, err := db.Exec("SET TIME ZONE 'UTC'")
	if err != nil {
		panic(err)
	}

	for _, model := range []interface{}{(*job.Job)(nil)} {
		err := db.CreateTable(model, &orm.CreateTableOptions{IfNotExists: true})
		if err != nil {
			panic(err)
		}
	}
}

func Query(response interface{}, query string, params ...interface{}) (pg.Result, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	res, err := db.Query(&response, query, params...)

	if err != nil {
		return res, err
	}

	return res, nil
}

func CreateOne(jd job.Job) (job.Job, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	jd.ID = ksuid.New().String()

	_, err := db.Model(&jd).Insert()
	if err != nil {
		return job.Job{}, err
	}

	return GetOne(jd.ID)
}

func GetOne(jobId string) (job.Job, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	jd := job.Job{}

	err := db.Model(&jd).Where("id = ?", jobId).Select()
	if err != nil {
		return jd, err
	}

	return jd, nil
}

func GetAll(jobServiceName string) ([]job.Job, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	var jds []job.Job

	err := db.Model(&jds).Where("service_name = ?", jobServiceName).Select()
	if err != nil {
		return jds, err
	}

	return jds, nil
}

func UpdateOne(updates job.Job) (job.Job, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()
	job, err := GetOne(updates.ID)

	if err != nil {
		return job, err
	}

	err = db.Update(&updates)
	if err != nil {
		return job, err
	}

	job, err = GetOne(updates.ID)

	return job, nil
}

func DeleteOne(jobId string) (job.Job, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})
	defer db.Close()

	job, err := GetOne(jobId)

	if err != nil {
		return job, err
	}

	_, err = db.Model(&job).Where("id = ?", job.ID).Delete()
	if err != nil {
		return job, err
	}

	return job, nil
}
