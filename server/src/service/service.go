package service

import (
	"context"
	"github.com/victorlenerd/scheduler0/server/src/utils"
)

type Service struct {
	Pool *utils.Pool
	Ctx  context.Context
}