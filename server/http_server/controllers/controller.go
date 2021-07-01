package controllers

import (
	"github.com/go-pg/pg"
	"scheduler0/server/process"
)

// Controller http request handlers
type Controller struct {
	DBConnection *pg.DB
	JobProcessor *process.JobProcessor
}
