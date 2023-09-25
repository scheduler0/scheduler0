package models

import "time"

type JobQueueLog struct {
	Id              uint64    `json:"id"`
	NodeId          uint64    `json:"nodeId"`
	LowerBoundJobId uint64    `json:"lowerBoundJobId"`
	UpperBoundJobId uint64    `json:"upperBoundJobId"`
	Version         uint64    `json:"version"`
	DateCreated     time.Time `json:"dateCreated"`
}

type JobQueueVersion struct {
	Id                  uint64    `json:"id"`
	Version             uint64    `json:"version"`
	NumberOfActiveNodes uint64    `json:"numberOfActiveNodes"`
	DateCreated         time.Time `json:"dateCreated"`
}
