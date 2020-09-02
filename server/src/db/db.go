package db

import (
	"cron-server/server/src/managers"
	"cron-server/server/src/models"
	"cron-server/server/src/utils"
	"errors"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"
)

const MaxConnections = 100

func CreateConnectionEnv(env string) (io.Closer, error) {
	var postgresCredentials utils.PostgresCredentials

	if env == "DEV" {
		postgresCredentials = *utils.GetPostgresCredentials(utils.EnvDev)
	} else if env == "TEST" {
		postgresCredentials = *utils.GetPostgresCredentials(utils.EnvTest)
	} else if env == "PROD" {
		postgresCredentials = *utils.GetPostgresCredentials(utils.EnvProd)
	} else {
		return nil, errors.New("environment was not provided")
	}

	return pg.Connect(&pg.Options{
		Addr:     postgresCredentials.Addr,
		User:     postgresCredentials.User,
		Password: postgresCredentials.Password,
		Database: postgresCredentials.Database,
	}), nil
}

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
			IfNotExists: true,
			FKConstraints: true,
		})
		if err != nil {
			panic(err)
		}
	}
}

func RunSQLMigrations(pool *utils.Pool) {
	conn, err := pool.Acquire()
	if err != nil {
		panic(err)
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	var sql []byte

	dbMigrationSQLPath, _ := filepath.Abs("../../src/db/migration.sql")

	sql, err = ioutil.ReadFile(dbMigrationSQLPath)
	if err != nil {
		panic(err)
	}

	if len(sql) > 0 {
		_, err = db.Exec(string(sql))
		if err != nil {
			panic(err)
		}
	}
}

func SeedDatabase(pool *utils.Pool) {
	credentialManager := managers.CredentialManager{}
	// Seed database

	credentials, err := credentialManager.GetAll(pool, 0, 1, "date_created")
	if err != nil {
		panic(err)
	}

	if len(credentials) < 1 {
		credentialManager.HTTPReferrerRestriction = "*"
		_, err = credentialManager.CreateOne(pool)
		log.Println("Created default credentials")
		if err != nil {
			panic(err)
		}
	}
}
