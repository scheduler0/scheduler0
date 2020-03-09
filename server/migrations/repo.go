// Database layer

package migrations

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

func Query(pool *Pool, response interface{}, query string, params ...interface{}) (pg.Result, error) {
	conn, err := pool.Acquire()
	misc.CheckErr(err)
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	res, err := db.Query(&response, query, params...)

	if err != nil {
		return res, err
	}
	return res, nil
}
