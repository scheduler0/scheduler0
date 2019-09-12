package misc

import (
	"os"
)

type PostgresCredentials struct {
	Addr     string
	User     string
	Password string
	Database string
}

type RedisCredentials struct {
	Addr string
}

func GetPort() string {
	port := os.Getenv("PORT")

	if len(port) == 0 {
		port = "9090"
	}

	return ":" + port
}

func GetPostgresCredentials() *PostgresCredentials {
	psgc := &PostgresCredentials{
		Addr:     "localhost:5432",
		Password: "postgres",
		Database: "postgres",
		User:     "postgres",
	}

	addr := os.Getenv("POSTGRES_ADDRESS")
	pass := os.Getenv("POSTGRES_PASSWORD")
	db := os.Getenv("POSTGRES_DATABASE")

	if len(addr) > 0 {
		psgc.Addr = addr
	}

	if len(pass) > 0 {
		psgc.Password = addr
	}

	if len(db) > 0 {
		psgc.Database = db
	}

	return psgc
}

func GetRedisCredentials() *RedisCredentials {
	rc := &RedisCredentials{Addr: "localhost:6379"}
	addr := os.Getenv("REDIS_ADDRESS")

	if len(addr) > 0 {
		rc.Addr = addr
	}

	return rc
}
