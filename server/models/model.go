package models

import (
	"cron-server/server/misc"
	"cron-server/server/repository"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// Basic model interface
type Model interface {
	SetId(id string)
	CreateOne(pool *repository.Pool) (string, error)
	GetOne(pool *repository.Pool, query string, params interface{}) error
	GetAll(pool *repository.Pool, query string, params ...string) ([]interface{}, error)
	UpdateOne(pool *repository.Pool) error
	DeleteOne(pool *repository.Pool) (int, error)
	FromJson(body []byte)
	SearchToQuery([][]string) (string, []string)
	ToJson() []byte
}

func Setup(pool *repository.Pool) {
	conn, err := pool.Acquire()
	misc.CheckErr(err)
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var runMigrations = func() {
		pwd, err := os.Getwd()
		misc.CheckErr(err)

		absPath, err := filepath.Abs(pwd + "/server/repository/migration.sql")
		misc.CheckErr(err)

		sql, err := ioutil.ReadFile(absPath)
		misc.CheckErr(err)

		if len(sql) > 0 {
			_, err = db.Exec(string(sql))
			misc.CheckErr(err)
		}
	}

	for _, model := range []interface{}{
		(*Job)(nil),
		(*Project)(nil),
		(*Credential)(nil),
	} {
		err := db.CreateTable(model, &orm.CreateTableOptions{IfNotExists: true})
		if err != nil {
			log.Println("Cannot to database")
		} else {
			runMigrations()
		}
	}
}
