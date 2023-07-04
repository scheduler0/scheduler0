package models

import "time"

type AsyncTaskState uint64

const (
	AsyncTaskNotStated  AsyncTaskState = 0
	AsyncTaskInProgress AsyncTaskState = 1
	AsyncTaskSuccess    AsyncTaskState = 2
	AsyncTaskFail       AsyncTaskState = 3
)

type AsyncTask struct {
	Id          uint64         `json:"id" fake:"{number:1,100}"`
	RequestId   string         `json:"requestId" fake:"{regex:[abcdef]{15}}"`
	Input       string         `json:"input" fake:"{regex:[abcdef]{15}}"`
	Output      string         `json:"output" fake:"{regex:[abcdef]{15}}"`
	Service     string         `json:"service" fake:"{regex:[abcdef]{15}}"`
	State       AsyncTaskState `json:"state" fake:"{number:1,100}"`
	DateCreated time.Time      `json:"dateCreated"`
}

type AsyncTaskRes struct {
	Data    AsyncTask `json:"data"`
	Success bool      `json:"success"`
}
