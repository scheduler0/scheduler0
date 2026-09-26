package node

import (
	"context"
	"scheduler0/pkg/models"
)

type Server interface {
	SetupTCPListener()
	BeginUncommittedLogsFetchRequest(data models.FetchRemoteData) models.String
	HandelUncommittedLogsFetchRequest(ctx context.Context, data models.FetchRemoteData) models.AsyncTask
}
