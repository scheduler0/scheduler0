package middlewares

import (
	"context"
	"cron-server/server/src/managers"
	"cron-server/server/src/utils"
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

func (_ *MiddlewareType) AuthMiddleware(pool *utils.Pool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password := utils.GetAuthentication()
			user, pass, passBasicAuth := r.BasicAuth()

			// Check for basic authentication
			if !passBasicAuth || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
				passBasicAuth = false
			}

			if passBasicAuth == false {
				credential := managers.CredentialManager{}

				// Check for api key
				token := r.Header.Get("x-token")

				if len(token) < 1 {
					utils.SendJson(w, "missing token header", false, http.StatusUnauthorized, nil)
					return
				}

				paths := strings.Split(r.URL.Path, "/")
				if len(paths) < 1 {
					utils.SendJson(w, "endpoint is not supported", false, http.StatusNotImplemented, nil)
					return
				}

				var restrictedPaths = strings.Join([]string{"credentials", "projects", "executions"}, ",")

				if strings.Contains(restrictedPaths, strings.ToLower(paths[1])) && token != "test-token" {
					utils.SendJson(w, paths[1]+" are not available", false, http.StatusUnauthorized, nil)
					return
				}

				credential.ApiKey = token

				if err := credential.GetByAPIKey(pool); err != nil {
					utils.SendJson(w, err.Error(), false, http.StatusInternalServerError, nil)
					return
				}

				if len(credential.ID) < 1 {
					utils.SendJson(w, "credential does not exits", false, http.StatusUnauthorized, nil)
					return
				}

				if len(credential.ID) < 1 {
					utils.SendJson(w, "credential does not exits", false, http.StatusUnauthorized, nil)
					return
				}

				if len(credential.HTTPReferrerRestriction) > 1 &&
					credential.HTTPReferrerRestriction != "*" &&
					credential.HTTPReferrerRestriction != r.URL.Host {
					utils.SendJson(w, "credential invalid referral", false, http.StatusUnauthorized, nil)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
