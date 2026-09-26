package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"scheduler0-private/pkg/db/migrations"
	"scheduler0-private/pkg/models"
	"scheduler0-private/pkg/utils"
	"slices"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/spf13/afero"
)

type dataStore struct {
	dbFilePath string
	fileLock   sync.Mutex

	isInMemDb      bool
	connectionLock sync.Mutex
	connection     *sql.DB

	logger hclog.Logger
}

type DataStore interface {
	OpenConnectionToExistingDB() io.Closer
	Serialize() []byte
	ConnectionLock()
	ConnectionUnlock()
	FileLock()
	FileUnlock()
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	GetOpenConnection() *sql.DB
	UpdateOpenConnection(conn *sql.DB)
	RunMigration(logger hclog.Logger)
	Backup(ctx context.Context) (string, error)
	Restore(ctx context.Context, filePath string) error
}

func NewSqliteDbConnection(logger hclog.Logger, dbFilePath string) DataStore {
	return &dataStore{
		dbFilePath: dbFilePath,
		logger:     logger,
		isInMemDb:  false,
	}
}

func (db *dataStore) OpenConnectionToExistingDB() io.Closer {
	db.fileLock.Lock()
	defer db.fileLock.Unlock()

	if db.connection != nil {
		return db.connection
	}

	once := sync.Once{}

	once.Do(func() {
		connection, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1", db.dbFilePath))
		if err != nil {
			db.logger.Error("failed to open db", err.Error())
		}

		db.connection = connection
	})

	return db.connection
}

func (db *dataStore) Serialize() []byte {
	db.fileLock.Lock()
	defer db.fileLock.Unlock()

	data, err := os.ReadFile(db.dbFilePath)
	if err != nil {
		db.logger.Error("Fatal error getting working dir: %s \n", err)
	}

	return data
}

func (db *dataStore) ConnectionLock() {
	db.connectionLock.Lock()
}

func (db *dataStore) ConnectionUnlock() {
	db.connectionLock.Unlock()
}

func (db *dataStore) FileLock() {
	db.fileLock.Lock()
}

func (db *dataStore) FileUnlock() {
	db.fileLock.Unlock()
}

func (db *dataStore) GetOpenConnection() *sql.DB {
	return db.connection
}

func (db *dataStore) UpdateOpenConnection(conn *sql.DB) {
	db.connection = conn
}

func (db *dataStore) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return db.connection.BeginTx(ctx, opts)
}

func CreateConnectionFromNewDbIfNonExists(logger hclog.Logger) DataStore {
	dirPath, filePath := utils.GetSqliteDbDirAndDbFilePath()
	fs := afero.NewOsFs()
	exists, err := afero.DirExists(fs, dirPath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error checking dir exist: %s \n", err))
	}

	if !exists {
		if mkErr := fs.MkdirAll(dirPath, os.ModePerm); mkErr != nil {
			log.Fatalln(fmt.Errorf("fatal error creating sqlite dir: %s", mkErr))
		}
	}

	if !exists {
		RunMigrations(logger, filePath)
	}

	sqliteDb := NewSqliteDbConnection(logger, filePath)
	conn := sqliteDb.OpenConnectionToExistingDB()

	dbConnection := conn.(*sql.DB)
	err = dbConnection.Ping()
	if err != nil {
		logger.Error("ping error: failed to create file db: %v", err)
	}

	return sqliteDb
}

func CreateConnectionFromNewDbIfNonExistsForNode(logger hclog.Logger, nodeId uint64) DataStore {
	dirPath, filePath := utils.GetSqliteDbDirAndDbFilePathForNode(nodeId)
	fs := afero.NewOsFs()
	exists, err := afero.DirExists(fs, dirPath)
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal error checking dir exist: %s \n", err))
	}

	if !exists {
		if mkErr := fs.MkdirAll(dirPath, os.ModePerm); mkErr != nil {
			log.Fatalln(fmt.Errorf("fatal error creating sqlite dir: %s", mkErr))
		}
	}

	if !exists {
		RunMigrations(logger, filePath)
	}

	sqliteDb := NewSqliteDbConnection(logger, filePath)
	conn := sqliteDb.OpenConnectionToExistingDB()

	dbConnection := conn.(*sql.DB)
	err = dbConnection.Ping()
	if err != nil {
		logger.Error("ping error: failed to create file db: %v", err)
	}

	return sqliteDb
}

func GetDBMEMConnection(logger hclog.Logger) DataStore {
	conn, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1", ":memory:"))
	if err != nil {
		logger.Error("ping error: failed to create in memory db: %v", err)
	}
	return &dataStore{
		isInMemDb:  true,
		connection: conn,
	}
}

func (db *dataStore) RunMigration(logger hclog.Logger) {
	RunMigrations(logger, db.dbFilePath)
}

func (db *dataStore) Backup(ctx context.Context) (string, error) {
	if db.isInMemDb {
		return "", fmt.Errorf("cannot backup in-memory database")
	}

	db.connectionLock.Lock()
	defer db.connectionLock.Unlock()

	if db.connection == nil {
		return "", fmt.Errorf("no active database connection")
	}

	backupPath := generateBackupPath(db.dbFilePath)

	db.logger.Info("performing backup", "path", backupPath)
	if err := db.backupToDestination(ctx, backupPath); err != nil {
		os.Remove(backupPath)
		db.logger.Error("backup failed", "error", err)
		return "", err
	}

	db.logger.Info("database backup completed", "path", backupPath)
	return backupPath, nil
}

func (db *dataStore) backupToDestination(ctx context.Context, destPath string) error {
	destDB, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1", destPath))
	if err != nil {
		db.logger.Error("failed to open destination database", "error", err)
		return fmt.Errorf("failed to open destination database: %w", err)
	}
	defer destDB.Close()

	if err := backupDatabase(ctx, db.logger, db.connection, destDB); err != nil {
		db.logger.Error("backup operation failed", "error", err)
		return fmt.Errorf("backup operation failed: %w", err)
	}

	return nil
}

func (db *dataStore) Restore(ctx context.Context, filePath string) error {
	if db.isInMemDb {
		return fmt.Errorf("cannot restore in-memory database")
	}

	db.connectionLock.Lock()
	defer db.connectionLock.Unlock()

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("backup file does not exist: %s", filePath)
	}

	if db.connection != nil {
		if err := db.connection.Close(); err != nil {
			db.logger.Warn("error closing connection before restore", "error", err)
		}
		db.connection = nil
	}

	currentBackupPath := db.dbFilePath + ".pre-restore-backup"
	if _, err := os.Stat(db.dbFilePath); err == nil {
		srcFile, err := os.Open(db.dbFilePath)
		if err != nil {
			return fmt.Errorf("failed to open current database for backup: %w", err)
		}
		defer srcFile.Close()

		dstFile, err := os.Create(currentBackupPath)
		if err != nil {
			return fmt.Errorf("failed to create pre-restore backup: %w", err)
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return fmt.Errorf("failed to copy current database to backup: %w", err)
		}
		db.logger.Info("created pre-restore backup", "path", currentBackupPath)
	}

	backupDB, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1&mode=ro", filePath))
	if err != nil {
		return fmt.Errorf("failed to open backup database: %w", err)
	}
	defer backupDB.Close()

	destDB, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1", db.dbFilePath))
	if err != nil {
		return fmt.Errorf("failed to open destination database: %w", err)
	}
	defer destDB.Close()

	if _, err := destDB.Exec("PRAGMA writable_schema=ON; DELETE FROM sqlite_master WHERE type IN ('table', 'index', 'trigger', 'view'); PRAGMA writable_schema=OFF;"); err != nil {
		rows, err := destDB.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
		if err == nil {
			var tables []string
			for rows.Next() {
				var name string
				rows.Scan(&name)
				tables = append(tables, name)
			}
			rows.Close()
			for _, table := range tables {
				destDB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
			}
		}
	}

	if err := backupDatabase(ctx, db.logger, backupDB, destDB); err != nil {
		db.logger.Error("restore failed, you can recover from pre-restore backup", "backup", currentBackupPath)
		return fmt.Errorf("restore operation failed: %w", err)
	}

	if err := destDB.Close(); err != nil {
		db.logger.Warn("error closing destination after restore", "error", err)
	}

	connection, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=1", db.dbFilePath))
	if err != nil {
		return fmt.Errorf("failed to re-open database after restore: %w", err)
	}

	db.connection = connection

	if err := db.connection.Ping(); err != nil {
		return fmt.Errorf("failed to ping database after restore: %w", err)
	}

	os.Remove(currentBackupPath)

	db.logger.Info("database restore completed", "from", filePath)
	return nil
}

var Migrations = []models.Migration{
	migrations.MainTables(),
	migrations.AICreditTables(),
	migrations.SeedAccountFeatures(),
	migrations.SeedSystemAccount(),
}

func RunMigrations(logger hclog.Logger, dbFilePath string) {
	datastore := NewSqliteDbConnection(logger, dbFilePath)
	conn := datastore.OpenConnectionToExistingDB()

	dbConnection := conn.(*sql.DB)
	log.Println("Creating migrations table")
	_, err := dbConnection.Exec("CREATE TABLE IF NOT EXISTS migrations (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)")
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal failed to create migrations table: %s", err))
	}

	log.Println("Querying migrations")
	rows, err := dbConnection.Query("SELECT name FROM migrations ORDER BY id")
	if err != nil {
		log.Fatalln(fmt.Errorf("Fatal failed to query migrations: %s", err))
	}

	var migrations []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Fatalln(fmt.Errorf("Fatal failed to scan migration name: %s", err))
		}
		migrations = append(migrations, name)
	}

	if err := rows.Err(); err != nil {
		log.Fatalln(fmt.Errorf("Fatal error iterating migrations: %s", err))
	}

	rows.Close()

	log.Println("Migrations: ", migrations)
	log.Println("Starting transaction")

	trx, dbConnErr := dbConnection.BeginTx(context.Background(), nil)
	if dbConnErr != nil {
		log.Println("open db transaction")
		log.Fatalln(fmt.Errorf("Fatl open db transaction error: %s", dbConnErr))
	}

	for _, migration := range Migrations {
		if slices.Contains(migrations, migration.Name) {
			log.Println("Migration already exists: ", migration.Name)
			continue
		}
		log.Println("Running migration: ", migration.Name)
		execErr := migration.Up(trx)
		log.Println("Inserting migration record: ", migration.Name)
		_, err := trx.Exec("INSERT INTO migrations (name) VALUES (?)", migration.Name)
		if err != nil {
			errRollback := trx.Rollback()
			if errRollback != nil {
				log.Fatalln(fmt.Errorf("Fatal rollback error: %s", err))
			}
			log.Fatalln(fmt.Errorf("Fatal failed to insert migration record: %s", err))
		}
		if execErr != nil {
			errRollback := trx.Rollback()
			if errRollback != nil {
				log.Fatalln(fmt.Errorf("Fatal rollback error: %s", execErr))
			}
			log.Fatalln(fmt.Errorf("Fatal closing db transaction error: %s", execErr))
		}
	}

	errCommit := trx.Commit()
	if errCommit != nil {
		log.Fatalln(fmt.Errorf("Fatal commit error: %s", errCommit))
	}
}

func generateBackupPath(dbFilePath string) string {
	timestamp := time.Now().Format("20060102-150405")
	dir := filepath.Dir(dbFilePath)
	baseName := filepath.Base(dbFilePath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := baseName[:len(baseName)-len(ext)]

	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0755)

	return filepath.Join(backupDir, fmt.Sprintf("%s-%s%s", nameWithoutExt, timestamp, ext))
}

func backupDatabase(
	ctx context.Context,
	logger hclog.Logger,
	srcDB, destDB *sql.DB,
) error {
	srcConn, err := srcDB.Conn(ctx)
	if err != nil {
		logger.Error("failed to get source connection", "error", err)
		return fmt.Errorf("get source connection: %w", err)
	}
	defer srcConn.Close()

	destConn, err := destDB.Conn(ctx)
	if err != nil {
		logger.Error("failed to get destination connection", "error", err)
		return fmt.Errorf("get destination connection: %w", err)
	}
	defer destConn.Close()

	return destConn.Raw(func(destDriverConn any) error {
		destSQLite, ok := destDriverConn.(*sqlite3.SQLiteConn)
		if !ok {
			logger.Error("destination driver conn is %T, expected *sqlite3.SQLiteConn", destDriverConn)
			return fmt.Errorf("destination driver conn is %T, expected *sqlite3.SQLiteConn", destDriverConn)
		}

		return srcConn.Raw(func(srcDriverConn any) error {
			srcSQLite, ok := srcDriverConn.(*sqlite3.SQLiteConn)
			if !ok {
				logger.Error("source driver conn is %T, expected *sqlite3.SQLiteConn", srcDriverConn)
				return fmt.Errorf("source driver conn is %T, expected *sqlite3.SQLiteConn", srcDriverConn)
			}

			bk, err := destSQLite.Backup("main", srcSQLite, "main")
			if err != nil {
				logger.Error("backup init: %w", err)
				return fmt.Errorf("backup init: %w", err)
			}
			defer func() {
				_ = bk.Finish()
			}()

			_, err = bk.Step(0)
			if err != nil {
				logger.Error("backup step(0): %w", err)
				return fmt.Errorf("backup step(0): %w", err)
			}

			total := bk.PageCount()
			for {
				done, err := bk.Step(64)
				if err != nil {
					logger.Error("backup step: %w", err)
					return fmt.Errorf("backup step: %w", err)
				}

				if done {
					break
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}

			if err := bk.Finish(); err != nil {
				logger.Error("backup finish: %w", err)
				return fmt.Errorf("backup finish: %w", err)
			}

			logger.Info("sqlite backup completed", "pages", total)
			return nil
		})
	})
}
