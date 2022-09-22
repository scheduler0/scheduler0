package migrate

import (
	"database/sql"
	_ "database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/exp/slices"
	_ "golang.org/x/exp/slices"
	"log"
	_ "log"
	"os"
	"strings"
)

const DB_BASE_PATH = ""
const MIGRATIONS_BASE_PATH = "/server/db/migrations"

type Migration struct {
	Id   int64
	Name string
}

type MigrationFile struct {
	Name string
}

type Migrator interface {
	OpenConnection()
	GetMigrations() ([]Migration, error)
	GetMigrationFiles() ([]MigrationFile, error)
	RunMigration(file MigrationFile)
	RunMigrations()
}

type Migrate struct {
	dbName       string
	basePath     string
	dbConnection *sql.DB
}

func NewMigrator(dbName string, basePath string) Migrator {
	return &Migrate{dbName: dbName, basePath: basePath}
}

func (m *Migrate) OpenConnection() {
	fmt.Println(m.basePath + m.dbName)
	dbConn, err := sql.Open("sqlite3", m.basePath+m.dbName)
	if err != nil {
		log.Fatalf("Failed to open connection for migration: %v", err.Error())
	}
	query := "CREATE TABLE IF NOT EXISTS migrations ( id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL);"
	_, err = dbConn.Exec(query)
	if err != nil {
		log.Fatalf("Failed to create migration table: %v", err.Error())
	}
	m.dbConnection = dbConn
}

func (m *Migrate) GetMigrations() ([]Migration, error) {
	migrations := []Migration{}
	rows, err := m.dbConnection.Query("select id, name from migrations")
	if err != nil {
		return migrations, err
	}
	defer rows.Close()
	for rows.Next() {
		var migration Migration
		err = rows.Scan(&migration.Id, &migration.Name)
		if err != nil {
			log.Fatal(err)
		}
		migrations = append(migrations, migration)
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	return migrations, err
}

func (m *Migrate) GetMigrationFiles() ([]MigrationFile, error) {
	dirEntry, err := os.ReadDir(m.basePath + "/migrations")
	if err != nil {
		log.Fatal(err)
	}
	log.Print(dirEntry)
	var migrationFiles = []MigrationFile{}
	for _, dir := range dirEntry {
		fileInfo, err := dir.Info()
		if err != nil {
			log.Fatal(err)
		}

		name := fileInfo.Name()
		if strings.Contains(name, ".sql") {
			migrationFiles = append(migrationFiles, MigrationFile{Name: name})
		}
		log.Println(fileInfo)
	}
	return migrationFiles, err
}

func (m *Migrate) RunMigration(file MigrationFile) {
	data, err := os.ReadFile(m.basePath + "/migrations/" + file.Name)
	if err != nil {
		log.Fatal(err)
	}
	_, err = m.dbConnection.Exec(string(data))
	if err != nil {
		log.Fatal(err)
	}
	_, err = m.dbConnection.Exec(fmt.Sprintf("insert into migrations (\"name\") values ('%s')", file.Name))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Successfully Ran Migration " + file.Name)
}

func (m *Migrate) RunMigrations() {

	migrations, err := m.GetMigrations()
	if err != nil {
		log.Fatal(err)
	}

	for _, migration := range migrations {
		log.Println(migration.Name)
	}

	migrationFiles, err := m.GetMigrationFiles()
	if err != nil {
		log.Fatal(err)
	}

	migrationFilesToRun := []MigrationFile{}

	for _, migrationFile := range migrationFiles {
		log.Println(migrationFile.Name)

		index := slices.IndexFunc(migrations, func(migration Migration) bool {
			if migration.Name == migrationFile.Name {
				return true
			}
			return false
		})

		if index == -1 {
			migrationFilesToRun = append(migrationFilesToRun, migrationFile)
		}
	}

	for _, migrationFileToRun := range migrationFilesToRun {
		m.RunMigration(migrationFileToRun)
	}
}
