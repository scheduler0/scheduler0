package service

import (
	"context"
	"github.com/go-pg/pg"
)

// Service service level abstraction
type Service struct {
	DBConnection *pg.DB
	Ctx  context.Context
}
