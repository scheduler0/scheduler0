package service

import (
	"context"
	"cron-server/server/src/utils"
)

type Service struct {
	Pool *utils.Pool
	Ctx  context.Context
}