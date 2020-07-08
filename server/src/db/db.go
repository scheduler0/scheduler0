package db

import (
	"cron-server/server/src/managers"
	"cron-server/server/src/misc"
	"cron-server/server/src/models"
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
	var postgresCredentials misc.PostgresCredentials

	if env == "DEV" {
		postgresCredentials = *misc.GetPostgresCredentials(misc.EnvDev)
	} else if env == "TEST" {
		postgresCredentials = *misc.GetPostgresCredentials(misc.EnvTest)
	} else if env == "PROD" {
		postgresCredentials = *misc.GetPostgresCredentials(misc.EnvProd)
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

func CreateModelTables(pool *misc.Pool) {
	conn, err := pool.Acquire()
	if err != nil {
		panic(err)
	}

	db := conn.(*pg.DB)
	defer pool.Release(conn)

	// Create tables
	for _, model := range []interface{}{
		(*models.JobModel)(nil),
		(*models.ProjectModel)(nil),
		(*models.CredentialModel)(nil),
		(*models.ExecutionModel)(nil),
	} {
		err := db.CreateTable(model, &orm.CreateTableOptions{IfNotExists: true})
		if err != nil {
			panic(err)
		}
	}
}

func RunSQLMigrations(pool *misc.Pool) {
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

func SeedDatabase(pool *misc.Pool) {
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
