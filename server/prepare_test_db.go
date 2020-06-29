package main

import (
	"cron-server/server/db"
	"cron-server/server/db/managers"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func seed(pool *db.Pool)  {
	var c = managers.CredentialManager{}

	// Seed database

	_, err := c.GetOne(pool)
	if err != nil {
		panic(err)
	}

	if len(c.ID) < 1 {
		c.HTTPReferrerRestriction = "*"
		_, err = c.CreateOne(pool)
		log.Println("Created default credentials")
		if err != nil {
			panic(err)
		}
	}
}

func migrations(db *pg.DB)  {
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

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
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	// Connect to database
	pool, err := db.NewPool(func() (closer io.Closer, err error) {
		return db.CreateConnectionEnv("TEST")
	}, 1)

	if err != nil {
		panic(err)
	}

	conn, err := pool.Acquire()
	if err != nil {
		panic(err)
	}


	db := conn.(*pg.DB)
	defer pool.Release(conn)

	// Create tables

	for _, model := range []interface{}{
		(*managers.JobManager)(nil),
		(*managers.ProjectManager)(nil),
		(*managers.CredentialManager)(nil),
		(*managers.ExecutionManager)(nil),
	} {
		err := db.CreateTable(model, &orm.CreateTableOptions{IfNotExists: true})
		if err != nil {
			panic(err)
		}
	}

	migrations(db)
	seed(pool)
}