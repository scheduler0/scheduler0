package middlewares

import (
	"context"
	"fmt"
	"github.com/segmentio/ksuid"
	"log"
	"net/http"
	"scheduler0/config"
	"scheduler0/headers"
	"scheduler0/node"
	"scheduler0/service"
	"scheduler0/utils"
	"strings"
	"sync"
)

const (
	RequestID = iota + 1
)

// middlewareHandler middleware type
type middlewareHandler struct {
	logger *log.Logger
	doOnce sync.Once
	ctx    context.Context
}

type MiddlewareHandler interface {
	ContextMiddleware(next http.Handler) http.Handler
	AuthMiddleware(credentialService service.Credential) func(next http.Handler) http.Handler
}

func NewMiddlewareHandler(logger *log.Logger) *middlewareHandler {
	return &middlewareHandler{
		logger: logger,
	}
}

// ContextMiddleware context middleware
func (m *middlewareHandler) ContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ksuid.New().String()
		ctx := r.Context()
		ctx = context.WithValue(ctx, RequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthMiddleware authentication middleware
func (m *middlewareHandler) AuthMiddleware(credentialService service.Credential) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			paths := strings.Split(r.URL.Path, "/")

			if len(paths) < 1 {
				utils.SendJSON(w, "endpoint is not supported", false, http.StatusNotImplemented, nil)
				return
			}

			if paths[1] == "api-docs" || paths[1] == "healthcheck" {
				next.ServeHTTP(w, r)
				return
			}

			if IsServerClient(r) {
				if validity, _ := IsAuthorizedServerClient(r, credentialService); validity {
					next.ServeHTTP(w, r)
					return
				} else {
					utils.SendJSON(w, "unauthorized requests", false, http.StatusUnauthorized, nil)
					return
				}
			}

			if IsPeerClient(r) {
				if validity := IsAuthorizedPeerClient(r, m.logger); validity {
					next.ServeHTTP(w, r)
					return
				} else {
					utils.SendJSON(w, "unauthorized requests", false, http.StatusUnauthorized, nil)
					return
				}
			}

			utils.SendJSON(w, "unauthorized requests", false, http.StatusUnauthorized, nil)
			return
		})
	}
}

func (m *middlewareHandler) EnsureRaftLeaderMiddleware(peer *node.Node) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			paths := strings.Split(r.URL.Path, "/")

			if len(paths) < 1 {
				utils.SendJSON(w, "endpoint is not supported", false, http.StatusNotImplemented, nil)
				return
			}

			if paths[1] == "peer-handshake" {
				next.ServeHTTP(w, r)
				return
			}

			if !peer.AcceptWrites && (r.Method == http.MethodPost || r.Method == http.MethodDelete || r.Method == http.MethodPut) {
				configs := config.GetConfigurations(m.logger)
				serverAddr, _ := peer.FsmStore.Raft.LeaderWithID()

				redirectUrl := ""

				for _, leaderPeer := range configs.Replicas {
					if leaderPeer.RaftAddress == string(serverAddr) {
						redirectUrl = leaderPeer.Address
						break
					}
				}

				if redirectUrl == "" {
					m.logger.Println("failed to get redirect url from replicas")
					utils.SendJSON(w, "service is unavailable", false, http.StatusServiceUnavailable, nil)
					return
				}

				redirectUrl = fmt.Sprintf("%s%s", redirectUrl, r.URL.Path)

				w.Header().Set("Location", redirectUrl)
				requester := r.Header.Get(headers.PeerHeader)

				if requester == headers.PeerHeaderCMDValue || requester == headers.PeerHeaderValue {
					m.logger.Println("Redirecting request to leader", redirectUrl)
					http.Redirect(w, r, redirectUrl, 301)
				} else {
					utils.SendJSON(w, nil, false, http.StatusFound, nil)
				}

				return
			}

			next.ServeHTTP(w, r)
			return
		})
	}
}
