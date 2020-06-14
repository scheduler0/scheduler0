package main

import (
	"cron-server/server/db"
	"cron-server/server/managers"
	"cron-server/server/misc"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func Setup(pool *db.Pool) {
	conn, err := pool.Acquire()
	misc.CheckErr(err)
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	for _, model := range []interface{}{
		(*managers.JobManager)(nil),
		(*managers.ProjectManager)(nil),
		(*managers.CredentialManager)(nil),
		(*managers.ExecutionManager)(nil),
	} {
		err := db.CreateTable(model, &orm.CreateTableOptions{IfNotExists: true})
		if err != nil {
			log.Printf("Cannot to database %v", err)
		}
	}

	pwd, err := os.Getwd()
	misc.CheckErr(err)

	var absPath string
	var sql []byte

	absPath, err = filepath.Abs(pwd + "/server/db/migration.sql")

	sql, err = ioutil.ReadFile(absPath)
	if err != nil {
		absPath, err = filepath.Abs(pwd + "/db/migration.sql")
		sql, err = ioutil.ReadFile(absPath)
		if err != nil {
			panic(err)
		}
	}

	if len(sql) > 0 {
		_, err = db.Exec(string(sql))
		misc.CheckErr(err)
	}

	var c = managers.CredentialManager{}

	// TODO: "date_created < ?", []string{"now()"}
	_, err = c.GetOne(pool)
	if err != nil {
		misc.CheckErr(err)
	}

	if len(c.ID) < 1 {
		c.HTTPReferrerRestriction = "*"
		// TODO: Fix syntax error
		_, err = c.CreateOne(pool)
		log.Println("Created default credentials")
		if err != nil {
			misc.CheckErr(err)
		}
	}
}
