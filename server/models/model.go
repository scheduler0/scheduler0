package models

import (
	"context"
	"cron-server/server/misc"
	"cron-server/server/repository"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
)

// Basic model interface
type Model interface {
	CreateOne(pool *repository.Pool, ctx context.Context) (string, error)
	GetOne(pool *repository.Pool, ctx context.Context, query string, params interface{}) error
	GetAll(pool *repository.Pool, ctx context.Context, query string, params ...string) ([]interface{}, error)
	UpdateOne(pool *repository.Pool, ctx context.Context) error
	DeleteOne(pool *repository.Pool, ctx context.Context) (int, error)
	SearchToQuery([][]string) (string, []string)
	FromJson(body []byte) error
	ToJson() ([]byte, error)
	SetId(id string)
}

func Setup(pool *repository.Pool) {
	conn, err := pool.Acquire()
	misc.CheckErr(err)
	db := conn.(*pg.DB)
	defer pool.Release(conn)

	for _, model := range []interface{}{
		(*Job)(nil),
		(*Project)(nil),
		(*Credential)(nil),
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

	absPath, err = filepath.Abs(pwd + "/server/repository/migration.sql")

	sql, err = ioutil.ReadFile(absPath)
	if err != nil {
		absPath, err = filepath.Abs(pwd + "/repository/migration.sql")
		sql, err = ioutil.ReadFile(absPath)
		if err != nil {
			panic(err)
		}
	}

	if len(sql) > 0 {
		_, err = db.Exec(string(sql))
		misc.CheckErr(err)
	}

	var c = Credential{}
	var ctx = context.Background()

	credentials, err := c.GetAll(pool, ctx, "date_created < ?", "now()")
	if err != nil {
		misc.CheckErr(err)
	}

	vd := reflect.ValueOf(credentials)
	credentialsWithName := make([]Credential, vd.Len())

	for i := 0; i < vd.Len(); i++ {
		credentialsWithName[i] = vd.Index(i).Interface().(Credential)
	}

	if len(credentialsWithName) < 1 {
		c.HTTPReferrerRestriction = "*"
		_, err := c.CreateOne(pool, ctx)
		log.Println("Created default credentials")
		if err != nil {
			misc.CheckErr(err)
		}
	}
}
