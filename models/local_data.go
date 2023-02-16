package models

type LocalData struct {
	AsyncTasks    []AsyncTask
	ExecutionLogs []JobExecutionLog
}

type CommitLocalData struct {
	Address string    `json:"address"`
	Data    LocalData `json:"data"`
}
