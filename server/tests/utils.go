package tests

import (
	"encoding/json"
	"fmt"
	"github.com/go-pg/pg"
	"io"
	"io/ioutil"
	"net/http/httptest"
	"scheduler0/server/src/db"
	"scheduler0/server/src/utils"
)

// GetTestPool returns a pool of connection to the database for tests
func GetTestPool() *utils.Pool {
	pool, err := utils.NewPool(func() (closer io.Closer, err error) {
		return db.CreateConnectionEnv("TEST")
	}, 1000)

	if err != nil {
		panic(err)
	}

	return pool
}

// Teardown is executed in tests to clear the database for stateless tests
func Teardown() {
	postgresCredentials := *utils.GetPostgresCredentials(utils.EnvTest)

	db := pg.Connect(&pg.Options{
		Addr:     postgresCredentials.Addr,
		User:     postgresCredentials.User,
		Password: postgresCredentials.Password,
		Database: postgresCredentials.Database,
	})
	defer db.Close()

	truncateQuery := "" +
		"TRUNCATE TABLE credentials;" +
		"TRUNCATE TABLE executions CASCADE;" +
		"TRUNCATE TABLE jobs CASCADE;" +
		"TRUNCATE TABLE projects CASCADE;"

	_, err := db.Exec(truncateQuery)

	if err != nil {
		fmt.Println("[ERROR]: Could not truncate tables: ", err.Error())
	}
}

// Prepare creates the tables, runs migrations and seeds
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

// ExtractResponse is used in tests for controllers
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
