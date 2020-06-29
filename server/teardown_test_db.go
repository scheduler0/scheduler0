package main

import (
	"cron-server/server/misc"
	"github.com/go-pg/pg"
)

func main()  {
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