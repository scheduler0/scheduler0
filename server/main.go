package main

import (
	"context"
	"cron-server/server/controllers"
	"cron-server/server/domains"
	"cron-server/server/middlewares"
	"cron-server/server/migrations"
	"cron-server/server/misc"
	"cron-server/server/process"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	pool, err := migrations.NewPool(migrations.CreateConnection, migrations.MaxConnections)
	misc.CheckErr(err)

	// Setup logging
	log.SetFlags(0)
	log.SetOutput(new(misc.LogWriter))

	// Set time zone, create database and run migrations
	SetupDB(pool)

	// Start process to execute cron-server jobs
	go process.Start(pool)

	// HTTP router setup
	router := mux.NewRouter()

	// Security middleware
	secureMiddleware := secure.New(secure.Options{FrameDeny: true})

	// Initialize controllers
	executionController := controllers.ExecutionController{Pool: *pool}
	jobController := controllers.JobController{Pool: *pool}
	projectController := controllers.ProjectController{Pool: *pool}
	credentialController := controllers.CredentialController{Pool: *pool}

	// Mount middleware
	middleware := middlewares.MiddlewareType{}

	router.Use(secureMiddleware.Handler)
	router.Use(mux.CORSMethodMiddleware(router))
	router.Use(middleware.ContextMiddleware)
	router.Use(middleware.AuthMiddleware(pool))

	// Executions Endpoint
	router.HandleFunc("/executions", executionController.List).Methods(http.MethodGet)
	router.HandleFunc("/executions/{id}", executionController.GetOne).Methods(http.MethodGet)

	// Credentials Endpoint
	router.HandleFunc("/credentials", credentialController.CreateOne).Methods(http.MethodPost)
	router.HandleFunc("/credentials", credentialController.List).Methods(http.MethodGet)
	router.HandleFunc("/credentials/{id}", credentialController.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/credentials/{id}", credentialController.UpdateOne).Methods(http.MethodPut)
	router.HandleFunc("/credentials/{id}", credentialController.DeleteOne).Methods(http.MethodDelete)

	// Job Endpoint
	router.HandleFunc("/jobs", jobController.CreateOne).Methods(http.MethodPost)
	router.HandleFunc("/jobs", jobController.List).Methods(http.MethodGet)
	router.HandleFunc("/jobs/{id}", jobController.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/jobs/{id}", jobController.UpdateOne).Methods(http.MethodPut)
	router.HandleFunc("/jobs/{id}", jobController.DeleteOne).Methods(http.MethodDelete)

	// Projects Endpoint
	router.HandleFunc("/projects", projectController.CreateOne).Methods(http.MethodPost)
	router.HandleFunc("/projects", projectController.List).Methods(http.MethodGet)
	router.HandleFunc("/projects/{id}", projectController.GetOne).Methods(http.MethodGet)
	router.HandleFunc("/projects/{id}", projectController.UpdateOne).Methods(http.MethodPut)
	router.HandleFunc("/projects/{id}", projectController.DeleteOne).Methods(http.MethodDelete)

	log.Println("Server is running on port", misc.GetPort(), misc.GetClientHost())
	err = http.ListenAndServe(misc.GetPort(), router)
	misc.CheckErr(err)
}

func SetupDB(pool *migrations.Pool) {
	conn, err := pool.Acquire()
	misc.CheckErr(err)
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	for _, model := range []interface{}{
		(*domains.JobDomain)(nil),
		(*domains.ProjectDomain)(nil),
		(*domains.CredentialDomain)(nil),
		(*domains.ExecutionDomain)(nil),
	} {
		err := db.CreateTable(model, &orm.CreateTableOptions{IfNotExists: true})
		if err != nil {
			log.Printf("Cannot to database %v", err)
		}
	}

	pwd, err := os.Getwd()
	misc.CheckErr(err)

	var absPath string
	var sql []byte

	absPath, err = filepath.Abs(pwd + "/server/migrations/migration.sql")

	sql, err = ioutil.ReadFile(absPath)
	if err != nil {
		absPath, err = filepath.Abs(pwd + "/migrations/migration.sql")
		sql, err = ioutil.ReadFile(absPath)
		if err != nil {
			panic(err)
		}
	}

	if len(sql) > 0 {
		_, err = db.Exec(string(sql))
		misc.CheckErr(err)
	}

	var c = domains.CredentialDomain{}
	var ctx = context.Background()



	// TODO: "date_created < ?", []string{"now()"}
	_, err = c.GetOne(pool, &ctx)
	if err != nil {
		misc.CheckErr(err)
	}

	if len(c.ID) < 1 {
		c.HTTPReferrerRestriction = "*"
		// TODO: Fix syntax error
		_, err = c.CreateOne(pool, &ctx)
		log.Println("Created default credentials")
		if err != nil {
			misc.CheckErr(err)
		}
	}
}