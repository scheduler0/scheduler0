package models

import (
	"time"
)

type JobStateLog struct {
	ExecutionTime     time.Time            `json:"executionTime,omitempty"`
	NodeId            uint64               `json:"nodeId,omitempty"`
	ServerHTTPAddress string               `json:"serverHTTPAddress"`
	State             JobExecutionLogState `json:"state,omitempty"`
	Data              []Job                `json:"data,omitempty"`
}
