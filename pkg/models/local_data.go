package models

type LocalData struct {
	AsyncTasks    []AsyncTask       `json:"asyncTasks"`
	ExecutionLogs []JobExecutionLog `json:"executionLogs"`
}

type CommitLocalData struct {
	Data LocalData `json:"data"`
}
