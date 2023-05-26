package models

type LocalData struct {
	AsyncTasks    []AsyncTask       `json:"asyncTasks"`
	ExecutionLogs []JobExecutionLog `json:"executionLogs"`
}

type CommitLocalData struct {
	Address string    `json:"address"`
	Data    LocalData `json:"data"`
}
