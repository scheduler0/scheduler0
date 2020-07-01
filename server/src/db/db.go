package db

import (
	"cron-server/server/src/misc"
	"errors"
	"github.com/go-pg/pg"
	"io"
	"sync"
)

type Pool struct {
	m         sync.Mutex
	resources chan io.Closer
	factory   func() (io.Closer, error)
	closed    bool
}

var ErrPoolClosed = errors.New("pool has been closed")

func NewPool(fn func() (io.Closer, error), size uint) (*Pool, error) {
	if size <= 0 {
		return nil, errors.New("size value too small")
	}

	return &Pool{
		resources: make(chan io.Closer, size),
		factory:   fn,
	}, nil
}

func (p *Pool) Acquire() (io.Closer, error) {
	select {
	// check if we have a connection in the pull
	case r, ok := <-p.resources:
		if !ok {
			return nil, ErrPoolClosed
		}
		return r, nil
	// otherwise create a new pool
	default:
		return p.factory()
	}
}

func (p *Pool) Release(r io.Closer) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.closed {
		r.Close()
		return
	}

	select {
	case p.resources <- r:
	default:
		r.Close()
	}
}

func (p *Pool) Close() {
	p.m.Lock()
	defer p.m.Unlock()

	if p.closed {
		return
	}

	p.closed = true

	close(p.resources)

	for r := range p.resources {
		r.Close()
	}
}

const MaxConnections = 100

func CreateConnectionEnv(env string) (io.Closer, error) {
	var postgresCredentials misc.PostgresCredentials

	if env == "DEV" {
		postgresCredentials = *misc.GetPostgresCredentials(misc.EnvDev)
	} else if env == "TEST" {
		postgresCredentials = *misc.GetPostgresCredentials(misc.EnvTest)
	} else if env == "PROD" {
		postgresCredentials = *misc.GetPostgresCredentials(misc.EnvProd)
	} else  {
		return nil, errors.New("environment was not provided")
	}

	return pg.Connect(&pg.Options{
		Addr:     postgresCredentials.Addr,
		User:     postgresCredentials.User,
		Password: postgresCredentials.Password,
		Database: postgresCredentials.Database,
	}), nil
}

