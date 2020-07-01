package tests

import (
	"cron-server/server/src/db"
	"cron-server/server/src/misc"
	"github.com/go-pg/pg"
	"io"
)

func GetTestPool() *misc.Pool {
	pool, err := misc.NewPool(func() (closer io.Closer, err error) {
		return db.CreateConnectionEnv("TEST")
	}, 1)

	if err != nil {
		panic(err)
	}

	return pool
}

func Teardown() {
	postgresCredentials := *misc.GetPostgresCredentials(misc.EnvTest)

	db := pg.Connect(&pg.Options{
		Addr:     postgresCredentials.Addr,
		User:     postgresCredentials.User,
		Password: postgresCredentials.Password,
		Database: postgresCredentials.Database,
	})
	defer db.Close()

	truncateQuery := "" +
		"TRUNCATE TABLE jobs;" +
		"TRUNCATE TABLE projects;" +
		"TRUNCATE TABLE credentials;" +
		"TRUNCATE TABLE executions;"

	_, err := db.Exec(truncateQuery)

	if err != nil {
		panic(err)
	}
}

func Prepare() {
	// Connect to database
	pool, err := misc.NewPool(func() (closer io.Closer, err error) {
		return db.CreateConnectionEnv("TEST")
	}, 1)

	if err != nil {
		panic(err)
	}

	db.CreateModelTables(pool)
	db.RunSQLMigrations(pool)
	db.SeedDatabase(pool)
}
