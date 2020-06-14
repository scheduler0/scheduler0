package middlewares

import (
	"context"
	"cron-server/server/managers"
	"cron-server/server/db"
	"cron-server/server/misc"
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

func (_ *MiddlewareType) AuthMiddleware(pool *db.Pool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			credential := managers.CredentialManager{}

			username, password := misc.GetAuthentication()
			user, pass, passBasicAuth := r.BasicAuth()

			// Check for basic authentication
			if !passBasicAuth || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
				passBasicAuth = false
			}

			// Check for api key
			token := r.Header.Get("x-token")
			if !passBasicAuth && len(token) < 1 {
				misc.SendJson(w, "missing token header", false, http.StatusUnauthorized, nil)
				return
			}

			paths := strings.Split(r.URL.Path, "/")
			if !passBasicAuth && len(paths) < 1 {
				misc.SendJson(w, "endpoint is not supported", false, http.StatusNotImplemented, nil)
				return
			}

			var restrictedPaths = strings.Join([]string{"credentials", "projects", "executions"}, ",")

			if !passBasicAuth && strings.Contains(restrictedPaths, strings.ToLower(paths[1])) {
				misc.SendJson(w, paths[1]+" are not available", false, http.StatusUnauthorized, nil)
				return
			}

			c, _ := credential.GetOne(pool)
			if !passBasicAuth && c < 1 {
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
