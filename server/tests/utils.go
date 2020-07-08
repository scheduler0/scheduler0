package tests

import (
	"cron-server/server/src/db"
	"cron-server/server/src/misc"
	"encoding/json"
	"github.com/go-pg/pg"
	"io"
	"io/ioutil"
	"net/http/httptest"
)

func GetTestPool() *misc.Pool {
	pool, err := misc.NewPool(func() (closer io.Closer, err error) {
		return db.CreateConnectionEnv("TEST")
	}, 100)

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

func ExtractResponse(w *httptest.ResponseRecorder) (*misc.Response, string, error) {
	body, err := ioutil.ReadAll(w.Body)

	if err != nil {
		return nil, "", err
	}

	res := &misc.Response{}

	err = json.Unmarshal(body, res)
	if err != nil {
		return nil, "", err
	}

	return res, string(body), nil
}