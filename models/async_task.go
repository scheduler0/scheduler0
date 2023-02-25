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
	Id          uint64         `json:"id"`
	RequestId   string         `json:"requestId"`
	Input       string         `json:"input"`
	Output      string         `json:"output"`
	Service     string         `json:"service"`
	State       AsyncTaskState `json:"state"`
	DateCreated time.Time      `json:"dateCreated"`
}

type AsyncTaskRes struct {
	Data    AsyncTask `json:"data"`
	Success bool      `json:"success"`
}
