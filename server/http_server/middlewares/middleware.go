package middlewares

import (
	"context"
	"github.com/segmentio/ksuid"
	"net/http"
	"scheduler0/server/http_server/middlewares/auth"
	"scheduler0/utils"
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
			paths := strings.Split(r.URL.Path, "/")
			if len(paths) < 1 {
				utils.SendJSON(w, "endpoint is not supported", false, http.StatusNotImplemented, nil)
				return
			}

			restrictedPaths := strings.Join([]string{"credentials", "projects", "executions"}, ",")
			isVisitingRestrictedPaths := strings.Contains(restrictedPaths, strings.ToLower(paths[1]))

			if isVisitingRestrictedPaths && auth.IsServerClient(r) {
				if validity, _ := auth.IsAuthorizedServerClient(r, pool); validity {
					next.ServeHTTP(w, r)
				}
			} else {
				if auth.IsIOSClient(r) {
					if validity, _ := auth.IsAuthorizedIOSClient(r, pool); validity {
						next.ServeHTTP(w, r)
						return
					}
				}

				if auth.IsAndroidClient(r) {
					if validity, _ := auth.IsAuthorizedAndroidClient(r, pool); validity {
						next.ServeHTTP(w, r)
						return
					}
				}

				if auth.IsWebClient(r)  {
					if validity, _ := auth.IsAuthorizedWebClient(r, pool); validity {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			utils.SendJSON(w, "unauthorized requests", false, http.StatusUnauthorized, nil)
			return
		})
	}
}
