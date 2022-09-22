package main

import (
	_ "github.com/mattn/go-sqlite3"
	"scheduler0/server/http_server"
)

func main() {
	//migrator := migrate.NewMigrator("/db.db", filepath.Join(os.Getenv("GOPATH"), "/src/scheduler0/server/db"))
	//migrator.OpenConnection()
	//migrator.RunMigrations()
	http_server.Start()
}
