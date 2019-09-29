// Database layer

package repository

import (
	"cron-server/server/misc"
	"cron-server/server/models"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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

	var runMigrations = func() {
		pwd, err := os.Getwd()
		misc.CheckErr(err)

		absPath, err := filepath.Abs(pwd + "/repository/migration.sql")
		misc.CheckErr(err)

		sql, err := ioutil.ReadFile(absPath)
		misc.CheckErr(err)

		if len(sql) > 0 {
			_, err = db.Exec(string(sql))
			misc.CheckErr(err)
		}
	}

	for _, model := range []interface{}{
		(*models.Job)(nil),
		(*models.Project)(nil),
		(*models.Credential)(nil),
	} {
		err := db.CreateTable(model, &orm.CreateTableOptions{IfNotExists: true})
		if err != nil {
			log.Println("Cannot to database")
			/*
				TODO: Send message to client that connection to database failed
					* Things to consider request would not be available when this happens
			*/
		} else {
			runMigrations()
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
