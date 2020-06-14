package testutils

import (
	"cron-server/server/misc"
	"github.com/go-pg/pg"
)

var psgc = misc.GetPostgresCredentials()

func TruncateDBBeforeTest() {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
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
