// Database layer

package db

import (
	"cron-server/server/misc"
	"github.com/go-pg/pg"
	"golang.org/x/tools/go/ssa/interp/testdata/src/errors"
	"io"
)

const MaxConnections = 100

func CreateConnectionEnv(env string) (io.Closer, error) {
	var postgresCredentials misc.PostgresCredentials

	if env == "DEV" {
		postgresCredentials = *misc.GetPostgresCredentials(misc.ENV_DEV)
	} else if env == "TEST" {
		postgresCredentials = *misc.GetPostgresCredentials(misc.ENV_TEST)
	} else if env == "PROD" {
		postgresCredentials = *misc.GetPostgresCredentials(misc.ENV_PROD)
	} else  {
		return nil, errors.New("Environment was not provided")
	}

	return pg.Connect(&pg.Options{
		Addr:     postgresCredentials.Addr,
		User:     postgresCredentials.User,
		Password: postgresCredentials.Password,
		Database: postgresCredentials.Database,
	}), nil
}
