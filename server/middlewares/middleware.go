package middlewares

import (
	"context"
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/repository"
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
				misc.SendJson(w, "missing token header", false, http.StatusUnauthorized, nil)
				return
			}

			paths := strings.Split(r.URL.Path, "/")
			if len(paths) < 1 {
				misc.SendJson(w, "endpoint is not supported", false, http.StatusBadRequest, nil)
				return
			}

			if paths[1] == "credentials" {
				misc.SendJson(w, "credentials are not available", false, http.StatusUnauthorized, nil)
			}

			err := credential.GetOne(pool, r.Context(), "api_key = ?", token)
			if err != nil {
				misc.SendJson(w, err.Error(), false, http.StatusInternalServerError, nil)
				return
			}

			if len(credential.ID) < 1 {
				misc.SendJson(w, "credential does not exits", false, http.StatusUnauthorized, nil)
				return
			}

			if len(credential.HTTPReferrerRestriction) > 1 &&
				credential.HTTPReferrerRestriction != "*" &&
				credential.HTTPReferrerRestriction != r.Host {
				misc.SendJson(w, "credential does not exits", false, http.StatusUnauthorized, nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
