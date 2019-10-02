package middlewares

import (
	"context"
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/repository"
	"errors"
	"github.com/segmentio/ksuid"
	"net/http"
	"strings"
	"sync"
)

const (
	RequestID = iota + 1
)

type MiddlewareType struct {
	doOnce sync.Once
	ctx    context.Context
}

func (m *MiddlewareType) ContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ksuid.New().String()
		ctx := r.Context()
		ctx = context.WithValue(ctx, RequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (_ *MiddlewareType) AuthMiddleware(pool *repository.Pool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			credential := models.Credential{}

			clientHost := r.Host
			if len(clientHost) > 1 && clientHost == misc.GetClientHost() {
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get("x-token")
			if len(token) < 1 {
				misc.SendJson(w, errors.New("missing token header"), http.StatusUnauthorized, nil)
			}

			paths := strings.Split(r.URL.Path, "/")

			if len(paths) < 1 {
				misc.SendJson(w, errors.New("endpoint is not supported"), http.StatusBadRequest, nil)
			}

			if paths[1] != "jobs" {
				misc.SendJson(w, errors.New("api request is limited to jobs endpoint"), http.StatusBadRequest, nil)
			}

			err := credential.GetOne(pool, r.Context(), "api_key = ?", token)
			if err != nil {
				misc.SendJson(w, err, http.StatusInternalServerError, nil)
			}

			if len(credential.ID) < 1 {
				misc.SendJson(w, errors.New("credential does not exits"), http.StatusUnauthorized, nil)
			}

			if len(credential.HTTPReferrerRestriction) > 1 &&
				credential.HTTPReferrerRestriction != "*" &&
				credential.HTTPReferrerRestriction != r.Host {
				misc.SendJson(w, errors.New("credential does not exits"), http.StatusUnauthorized, nil)
			}

			next.ServeHTTP(w, r)
		})
	}
}

var Middleware MiddlewareType
