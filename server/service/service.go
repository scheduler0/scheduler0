package service

import (
	"context"
	"scheduler0/utils"
)

// Service service level abstraction
type Service struct {
	Pool *utils.Pool
	Ctx  context.Context
}
