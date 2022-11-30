package models

import (
	"scheduler0/constants"
	"time"
)

type JobStateLog struct {
	ExecutionTime time.Time         `json:"executionTime,omitempty"`
	ServerAddress string            `json:"server_address,omitempty"`
	State         constants.Command `json:"state,omitempty"`
	Data          []JobModel        `json:"data,omitempty"`
}
