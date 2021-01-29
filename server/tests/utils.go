package tests

import (
	"github.com/victorlenerd/scheduler0/server/src/db"
	"github.com/victorlenerd/scheduler0/server/src/utils"
	"encoding/json"
	"fmt"
	"github.com/go-pg/pg"
	"io"
	"io/ioutil"
	"net/http/httptest"
)

func GetTestPool() *utils.Pool {
	pool, err := utils.NewPool(func() (closer io.Closer, err error) {
		return db.CreateConnectionEnv("TEST")
	}, 100)

	if err != nil {
		panic(err)
	}

	return pool
}

func Teardown() {
	postgresCredentials := *utils.GetPostgresCredentials(utils.EnvTest)

	fmt.Println(postgresCredentials)

	db := pg.Connect(&pg.Options{
		Addr:     postgresCredentials.Addr,
		User:     postgresCredentials.User,
		Password: postgresCredentials.Password,
		Database: postgresCredentials.Database,
	})
	defer db.Close()

	truncateQuery := "" +
		"TRUNCATE TABLE credentials CASCADE;" +
		"TRUNCATE TABLE executions CASCADE;" +
		"TRUNCATE TABLE jobs CASCADE;" +
		"TRUNCATE TABLE projects CASCADE;"

	_, err := db.Exec(truncateQuery)

	if err != nil {
		fmt.Println("[ERROR]: Could not truncate tables: ", err.Error())
	}
}

func Prepare() {
	// Connect to database
	pool, err := utils.NewPool(func() (closer io.Closer, err error) {
		return db.CreateConnectionEnv("TEST")
	}, 1)

	if err != nil {
		panic(err)
	}

	db.CreateModelTables(pool)
	//db.RunSQLMigrations(pool)
	db.SeedDatabase(pool)
}

func ExtractResponse(w *httptest.ResponseRecorder) (*utils.Response, string, error) {
	body, err := ioutil.ReadAll(w.Body)

	if err != nil {
		return nil, "", err
	}

	res := &utils.Response{}

	err = json.Unmarshal(body, res)
	if err != nil {
		return nil, "", err
	}

	return res, string(body), nil
}