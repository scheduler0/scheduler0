package middlewares

import (
	"context"
	"github.com/segmentio/ksuid"
	"net/http"
	"scheduler0/server/http_server/middlewares/auth/android"
	"scheduler0/server/http_server/middlewares/auth/ios"
	"scheduler0/server/http_server/middlewares/auth/server"
	"scheduler0/server/http_server/middlewares/auth/web"
	"scheduler0/utils"
	"strings"
	"sync"
)

const (
	RequestID = iota + 1
)

// MiddlewareType middleware type
type MiddlewareType struct {
	doOnce sync.Once
	ctx    context.Context
}

// ContextMiddleware context middleware
func (m *MiddlewareType) ContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ksuid.New().String()
		ctx := r.Context()
		ctx = context.WithValue(ctx, RequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthMiddleware authentication middleware
func (_ *MiddlewareType) AuthMiddleware(pool *utils.Pool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			paths := strings.Split(r.URL.Path, "/")
			if len(paths) < 1 {
				utils.SendJSON(w, "endpoint is not supported", false, http.StatusNotImplemented, nil)
				return
			}

			restrictedPaths := []string{"credentials", "projects", "executions"}

			matchRestrictedPaths := func(path string) bool {
				for _, restrictedPath := range restrictedPaths {
					if restrictedPath == path {
						return true
					}
				}
				return false
			}

			isVisitingRestrictedPaths := matchRestrictedPaths(strings.ToLower(paths[1]))

			if isVisitingRestrictedPaths && server.IsServerClient(r) {
				if validity, _ := server.IsAuthorizedServerClient(r, pool); validity {
					next.ServeHTTP(w, r)
					return
				}
			} else {
				if server.IsServerClient(r) {
					if validity, _ := ios.IsAuthorizedIOSClient(r, pool); validity {
						next.ServeHTTP(w, r)
						return
					}
				}

				if ios.IsIOSClient(r) {
					if validity, _ := ios.IsAuthorizedIOSClient(r, pool); validity {
						next.ServeHTTP(w, r)
						return
					}
				}

				if android.IsAndroidClient(r) {
					if validity, _ := android.IsAuthorizedAndroidClient(r, pool); validity {
						next.ServeHTTP(w, r)
						return
					}
				}

				if web.IsWebClient(r) {
					if validity, _ := web.IsAuthorizedWebClient(r, pool); validity {
						next.ServeHTTP(w, r)
						return
					}
				}

				if paths[1] == "api-docs" {
					next.ServeHTTP(w, r)
					return
				}
			}

			utils.SendJSON(w, "unauthorized requests", false, http.StatusUnauthorized, nil)
			return
		})
	}
}
