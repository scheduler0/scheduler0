package service

import (
	"context"
	"scheduler0/utils"
)

type Service struct {
	Pool *utils.Pool
	Ctx  context.Context
}
