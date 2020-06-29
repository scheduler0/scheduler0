package testutils

import (
	"cron-server/server/db"
	"cron-server/server/misc"
	"github.com/go-pg/pg"
	"io"
)

func GetTestDBPool() (*db.Pool, error) {
	pool, err := db.NewPool(func() (closer io.Closer, err error) {
		return db.CreateConnectionEnv("TEST")
	}, 10)

	if err != nil {
		panic(err)
	}

	return pool, nil
}


func TruncateDBAfterTest() {
	postgresCredentials := *misc.GetPostgresCredentials(misc.ENV_TEST)

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
	misc.CheckErr(err)
}
