package models

import "time"

type JobQueueLog struct {
	Id              uint64    `json:"id" fake:"{number:1,100}"`
	NodeId          uint64    `json:"nodeId" fake:"{number:1,100}"`
	LowerBoundJobId uint64    `json:"lowerBoundJobId" fake:"{number:1,100}"`
	UpperBoundJobId uint64    `json:"upperBoundJobId" fake:"{number:1,100}"`
	Version         uint64    `json:"version" fake:"{number:1,100}"`
	DateCreated     time.Time `json:"dateCreated" fake:"{date}"`
}

type JobQueueVersion struct {
	Id                  uint64    `json:"id" fake:"{number:1,100}"`
	Version             uint64    `json:"version" fake:"{number:1,100}"`
	NumberOfActiveNodes uint64    `json:"numberOfActiveNodes" fake:"{number:1,100}"`
	DateCreated         time.Time `json:"dateCreated" fake:"{date}"`
}
