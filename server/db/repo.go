// Database layer

package db

import (
	"cron-server/server/misc"
	"github.com/go-pg/pg"
	"io"
)

var psgc = misc.GetPostgresCredentials()

const MaxConnections = 100

func CreateConnection() (io.Closer, error) {
	db := pg.Connect(&pg.Options{
		Addr:     psgc.Addr,
		User:     psgc.User,
		Password: psgc.Password,
		Database: psgc.Database,
	})

	return db, nil
}
