package db

import (
	"fmt"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"io"
	"scheduler0/server/models"
	"scheduler0/utils"
)

const MaxConnections = 100

// OpenConnection opens a database connection with one pool
func OpenConnection() (io.Closer, error) {
	postgresCredentials := *utils.GetScheduler0Configurations()
	return pg.Connect(&pg.Options{
		Addr:     postgresCredentials.PostgresAddress,
		User:     postgresCredentials.PostgresUser,
		Password: postgresCredentials.PostgresPassword,
		Database: postgresCredentials.PostgresDatabase,
		PoolSize: MaxConnections,
	}), nil
}

// CreateModelTables this will create the tables needed
func CreateModelTables(dbConnection *pg.DB) {
	// Create tables
	for _, model := range []interface{}{
		(*models.CredentialModel)(nil),
		(*models.ProjectModel)(nil),
		(*models.JobModel)(nil),
		(*models.ExecutionModel)(nil),
	} {
		err := dbConnection.CreateTable(model, &orm.CreateTableOptions{
			IfNotExists:   true,
			FKConstraints: true,
		})
		if err != nil {
			utils.Error(err.Error())
		}
	}
}

// GetTestDBConnection returns a pool of connection to the database for tests
func GetTestDBConnection() *pg.DB {
	conn, err := OpenConnection()

	if err != nil {
		panic(err)
	}

	return conn.(*pg.DB)
}

// Teardown is executed in tests to clear the database for stateless tests
func Teardown() {
	postgresCredentials := *utils.GetScheduler0Configurations()

	db := pg.Connect(&pg.Options{
		Addr:     postgresCredentials.PostgresAddress,
		User:     postgresCredentials.PostgresUser,
		Password: postgresCredentials.PostgresPassword,
		Database: postgresCredentials.PostgresDatabase,
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
	postgresCredentials := *utils.GetScheduler0Configurations()

	// Connect to database
	db := pg.Connect(&pg.Options{
		Addr:     postgresCredentials.PostgresAddress,
		User:     postgresCredentials.PostgresUser,
		Password: postgresCredentials.PostgresPassword,
		Database: postgresCredentials.PostgresDatabase,
	})
	defer db.Close()

	CreateModelTables(db)
}
