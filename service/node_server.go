package service

import (
	"context"
	"scheduler0/models"
)

type NodeServer interface {
	SetupTCPListener()
	BeginUncommittedLogsFetchRequest(data models.FetchRemoteData) models.String
	HandelUncommittedLogsFetchRequest(ctx context.Context, data models.FetchRemoteData) models.AsyncTask
}
