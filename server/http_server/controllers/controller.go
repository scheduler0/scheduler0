package controllers

import (
	"scheduler0/server/process"
	"scheduler0/utils"
)

type Controller struct {
	Pool *utils.Pool
	JobProcessor *process.JobProcessor
}
