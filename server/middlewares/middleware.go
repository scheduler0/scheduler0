package middlewares

import (
	"context"
	"cron-server/server/misc"
	"cron-server/server/models"
	"cron-server/server/repository"
	"crypto/subtle"
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

			username, password := misc.GetAuthentication()
			user, pass, passBasicAuth := r.BasicAuth()

			if !passBasicAuth || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
				passBasicAuth = false
			}

			token := r.Header.Get("x-token")
			if !passBasicAuth && len(token) < 1 {
				misc.SendJson(w, "missing token header", false, http.StatusUnauthorized, nil)
				return
			}

			paths := strings.Split(r.URL.Path, "/")
			if !passBasicAuth && len(paths) < 1 {
				misc.SendJson(w, "endpoint is not supported", false, http.StatusBadRequest, nil)
				return
			}

			if !passBasicAuth && paths[1] == "credentials" {
				misc.SendJson(w, "credentials are not available", false, http.StatusUnauthorized, nil)
				return
			}

			err := credential.GetOne(pool, r.Context(), "api_key = ?", token)
			if !passBasicAuth && err != nil {
				misc.SendJson(w, "credential does not exits", false, http.StatusUnauthorized, nil)
				return
			}

			if !passBasicAuth && len(credential.ID) < 1 {
				misc.SendJson(w, "credential does not exits", false, http.StatusUnauthorized, nil)
				return
			}

			if !passBasicAuth && len(credential.HTTPReferrerRestriction) > 1 &&
				credential.HTTPReferrerRestriction != "*" &&
				credential.HTTPReferrerRestriction != r.URL.Host {
				misc.SendJson(w, "credential invalid referral", false, http.StatusUnauthorized, nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}