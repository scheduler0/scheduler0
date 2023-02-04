package models

import "time"

type AsyncTaskState uint64

const (
	AsyncTaskNotStated  AsyncTaskState = 0
	AsyncTaskInProgress                = 1
	AsyncTaskSuccess                   = 2
	AsyncTaskFail                      = 3
)

type AsyncTask struct {
	Id          uint64
	RequestId   string
	Input       string
	Output      string
	Service     string
	State       AsyncTaskState
	DateCreated time.Time
}
