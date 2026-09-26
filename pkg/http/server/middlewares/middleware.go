package middlewares

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants/headers"
	"scheduler0/pkg/secrets"
	"scheduler0/pkg/service/credential"
	"scheduler0/pkg/service/etcd"
	"scheduler0/pkg/service/node"
	"scheduler0/pkg/utils"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/raft"
	"github.com/segmentio/ksuid"
)

// clusterSubpathRequiresRaftLeader is true for cluster routes that must run on the Raft leader
// so backup/restore see consistent SQLite and S3 artifacts (same behavior as other POST writes).
func clusterSubpathRequiresRaftLeader(r *http.Request, paths []string) bool {
	if len(paths) < 5 || paths[3] != "cluster" {
		return false
	}
	if r.Method != http.MethodPost {
		return false
	}
	switch paths[4] {
	case "backup", "restore":
		return true
	default:
		return false
	}
}

// middlewareHandler middleware type
type middlewareHandler struct {
	logger           *log.Logger
	scheduler0Secret secrets.Scheduler0Secrets
	scheduler0Config config.Scheduler0Config
	etcdService      etcd.EtcdService
}

type MiddlewareHandler interface {
	ContextMiddleware(next http.Handler) http.Handler
	AuthMiddleware(credentialService credential.CredentialService) func(next http.Handler) http.Handler
	EnsureRaftLeaderMiddleware(peer node.NodeService) func(next http.Handler) http.Handler
	AccountIDMiddleware(next http.Handler) http.Handler
}

func NewMiddlewareHandler(logger *log.Logger, scheduler0Secret secrets.Scheduler0Secrets, scheduler0Config config.Scheduler0Config, etcdService etcd.EtcdService) MiddlewareHandler {
	return &middlewareHandler{
		logger:           logger,
		scheduler0Secret: scheduler0Secret,
		scheduler0Config: scheduler0Config,
		etcdService:      etcdService,
	}
}

// ContextMiddleware context middleware
func (m *middlewareHandler) ContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ksuid.New().String()
		ctx := r.Context()
		// Use the context key from utils package to ensure compatibility
		ctx = context.WithValue(ctx, utils.RequestIDContextKey(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthMiddleware authentication middleware
func (m *middlewareHandler) AuthMiddleware(credentialService credential.CredentialService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			paths := strings.Split(r.URL.Path, "/")

			// Check if path has enough segments (expecting /api/v1/endpoint format)
			if len(paths) < 4 {
				utils.SendJSON(w, "endpoint is not supported", false, http.StatusNotImplemented, nil)
				return
			}

			if paths[3] == "healthcheck" {
				next.ServeHTTP(w, r)
				return
			}

			// API-key credentials authenticate here, including for account- and
			// cluster-level routes: those now require the admin scope (enforced via
			// requiredScopeForRequest) rather than being peer-only. Peer/basic auth
			// remains available below as the operator bootstrap path.
			if IsServerClient(r) {
				validity, cred, _ := IsAuthorizedServerClient(r, credentialService)
				if validity && cred != nil {
					if cred.IsExpired(time.Now()) {
						utils.SendJSON(w, "credential expired", false, http.StatusUnauthorized, nil)
						return
					}
					required := requiredScopeForRequest(r)
					if !credentialSatisfiesRequiredScope(cred, required) {
						utils.SendJSON(w, "credential missing required scope: "+required, false, http.StatusForbidden, nil)
						return
					}
					ctx := context.WithValue(r.Context(), utils.CredentialContextKey(), cred)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			if IsPeerClient(r) {
				if validity := IsAuthorizedPeerClient(r, m.scheduler0Secret); validity {
					// A trusted peer may act on behalf of one of the account's API
					// credentials (X-Act-As-API-Key). The credential is then subject to
					// exactly the same archived/expiry/scope rules as a direct api-key
					// call, so the dashboard playground can exercise a credential
					// without ever holding its secret.
					if actAsKey := strings.TrimSpace(r.Header.Get(headers.ActAsAPIKeyHeader)); actAsKey != "" {
						cred, status, msg := resolveActAsCredential(r, actAsKey, credentialService)
						if cred == nil {
							utils.SendJSON(w, msg, false, status, nil)
							return
						}
						ctx := context.WithValue(r.Context(), utils.CredentialContextKey(), cred)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					next.ServeHTTP(w, r)
					return
				}
			}

			utils.SendJSON(w, "unauthorized request - cannot determine authentication type", false, http.StatusUnauthorized, nil)
		})
	}
}

// EnsureRaftLeaderMiddleware ensures that the current node is the leader of the raft cluster
func (m *middlewareHandler) EnsureRaftLeaderMiddleware(peer node.NodeService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			paths := strings.Split(r.URL.Path, "/")

			// Check if path has enough segments (expecting /api/v1/endpoint format)
			if len(paths) < 4 {
				utils.SendJSON(w, "endpoint is not supported", false, http.StatusNotImplemented, nil)
				return
			}

			if paths[3] == "cluster" && !clusterSubpathRequiresRaftLeader(r, paths) {
				next.ServeHTTP(w, r)
				return
			}

			if paths[3] == "peer-handshake" {
				next.ServeHTTP(w, r)
				return
			}

			if !peer.CanAcceptRequest() {
				utils.SendJSON(w, "peer cannot accept requests", false, http.StatusServiceUnavailable, nil)
				return
			}

			if !peer.CanAcceptClientWriteRequest() && (r.Method == http.MethodPost || r.Method == http.MethodDelete || r.Method == http.MethodPut) {
				leaderAddress, serverId := peer.GetRaftLeaderWithId()

				m.logger.Println("leader address", "leaderAddress", leaderAddress, "serverId", string(serverId))
				redirectUrl := ""

				nodeServiceId := raft.ServerID(strconv.FormatUint(m.scheduler0Config.GetConfigurations().NodeId, 10))

				if serverId == nodeServiceId {
					utils.SendJSON(w, "service is unavailable - node is the leader but cannot accept requests yet", false, http.StatusServiceUnavailable, nil)
					return
				}

				// Resolve leader client address from etcd node metadata
				if m.etcdService != nil {
					// Use request context so etcd calls are tied to this request lifetime.
					peers, pErr := m.etcdService.GetPeers(r.Context())
					if pErr == nil {
						// Convert raft.ServerID (string) to uint64 for comparison
						leaderNodeId, err := strconv.ParseUint(string(serverId), 10, 64)
						if err == nil {
							fmt.Println("leaderNodeId", leaderNodeId)
							for _, leaderPeer := range peers {
								if leaderPeer.NodeId == leaderNodeId {
									fmt.Println("leaderPeer", leaderPeer)
									if leaderPeer.ClientAddress != "" {
										redirectUrl = leaderPeer.ClientAddress
										fmt.Println("setting redirectUrl", redirectUrl)
									}
									break
								}
							}
						} else {
							m.logger.Println("failed to parse leader node id", "error", err)
							utils.SendJSON(w, "service is unavailable - failed to parse leader node id in etcd", false, http.StatusServiceUnavailable, nil)
							return
						}
					} else {
						m.logger.Println("failed to get peers from etcd", "error", pErr)
						utils.SendJSON(w, "service is unavailable - failed to get peers from etcd", false, http.StatusServiceUnavailable, nil)
						return
					}
				} else {
					utils.SendJSON(w, "service is unavailable - etcd service is not initialized", false, http.StatusServiceUnavailable, nil)
					return
				}

				if redirectUrl == "" {
					utils.SendJSON(w, "service is unavailable - failed to find leader node in etcd", false, http.StatusServiceUnavailable, nil)
					return
				}

				redirectUrl = fmt.Sprintf("%s%s", redirectUrl, r.URL.Path)

				w.Header().Set("Location", redirectUrl)
				requester := r.Header.Get(headers.PeerHeader)

				if requester == headers.PeerHeaderCMDValue || requester == headers.PeerHeaderValue {
					m.logger.Println("Redirecting request to leader", redirectUrl)
					http.Redirect(w, r, redirectUrl, http.StatusMovedPermanently)
				} else {
					utils.SendJSON(w, nil, false, http.StatusFound, nil)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AccountIDMiddleware parses the X-Account-ID header and stores it in the request
// context so handlers can retrieve it via utils.GetAccountID.
//
// For endpoints in the required list, a missing or invalid header is rejected
// with 400. For all other endpoints (e.g. /account/ai-settings called by a
// trusted peer), the header is parsed opportunistically: if it is present and
// valid it is stored in context; if absent the request proceeds unchanged.
func (m *middlewareHandler) AccountIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths := strings.Split(r.URL.Path, "/")
		if len(paths) < 4 {
			next.ServeHTTP(w, r)
			return
		}

		endpoint := paths[3]
		subpath := strings.Join(paths[4:], "/")

		requiresAccountID := endpoint == "jobs" ||
			endpoint == "projects" ||
			endpoint == "credentials" ||
			endpoint == "executors" ||
			endpoint == "async-tasks" ||
			endpoint == "executions" ||
			// AI action endpoints (but not the ai/models catalog, ai/settings,
			// or ai/prompt-requests, which resolve the account differently).
			(endpoint == "ai" && (subpath == "prompt" ||
				subpath == "prompt/classify" ||
				subpath == "suggestions/analyze" ||
				subpath == "suggestions/time" ||
				subpath == "schedule"))

		accountIDHeader := strings.TrimSpace(r.Header.Get(headers.AccountIDHeader))

		if accountIDHeader != "" {
			accountId, err := strconv.ParseUint(accountIDHeader, 10, 64)
			if err != nil || accountId == 0 {
				if requiresAccountID {
					utils.SendJSON(w, "invalid x-account-id header", false, http.StatusBadRequest, nil)
					return
				}
			} else {
				ctx := context.WithValue(r.Context(), utils.AccountIDContextKey(), accountId)
				r = r.WithContext(ctx)
				m.logger.Println("Request account id", accountIDHeader)
			}
		} else if requiresAccountID {
			utils.SendJSON(w, "x-account-id header is required", false, http.StatusBadRequest, nil)
			return
		}

		next.ServeHTTP(w, r)
	})
}
