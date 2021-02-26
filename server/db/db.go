package db

import (
	"fmt"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"io"
	"scheduler0/server/managers/credential"
	"scheduler0/server/models"
	"scheduler0/utils"
)

const MaxConnections = 100

// OpenConnection
func OpenConnection() (io.Closer, error) {
	postgresCredentials := *utils.GetPostgresCredentials()
	return pg.Connect(&pg.Options{
		Addr:     postgresCredentials.Addr,
		User:     postgresCredentials.User,
		Password: postgresCredentials.Password,
		Database: postgresCredentials.Database,
	}), nil
}

// CreateModelTables this will create the tables needed
func CreateModelTables(pool *utils.Pool) {
	conn, err := pool.Acquire()
	if err != nil {
		panic(err)
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	// Create tables
	for _, model := range []interface{}{
		(*models.CredentialModel)(nil),
		(*models.ProjectModel)(nil),
		(*models.JobModel)(nil),
		(*models.ExecutionModel)(nil),
	} {
		err := db.CreateTable(model, &orm.CreateTableOptions{
			IfNotExists:   true,
			FKConstraints: true,
		})
		if err != nil {
			utils.Error(err.Error())
		}
	}
}

// CreateDefaults this will create a default credential
func CreateDefaults(pool *utils.Pool) {
	credentialManager := credential.CredentialManager{}
	// Seed database

	credentials, err := credentialManager.GetAll(pool, 0, 1, "date_created")
	if err != nil {
		utils.Error(err.Message)
		panic(err.Message)
	}

	if len(credentials) < 1 {
		credentialManager.HTTPReferrerRestriction = "*"
		_, createCredentialError := credentialManager.CreateOne(pool)
		if createCredentialError != nil {
			panic(createCredentialError)
		}
	}
}

// GetTestPool returns a pool of connection to the database for tests
func GetTestPool() *utils.Pool {
	pool, err := utils.NewPool(OpenConnection, 1000)

	if err != nil {
		panic(err)
	}

	return pool
}

// Teardown is executed in tests to clear the database for stateless tests
func Teardown() {
	postgresCredentials := *utils.GetPostgresCredentials()

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
		return OpenConnection()
	}, 1)

	if err != nil {
		panic(err)
	}
	CreateModelTables(pool)
	CreateDefaults(pool)
}