package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"scheduler0/pkg/config"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdService interface {
	RegisterNode(nodeId uint64, nodeInfo config.RaftNode) error
	WatchPeers(ctx context.Context) (<-chan []config.RaftNode, error)
	GetPeers(ctx context.Context) ([]config.RaftNode, error)
	GetPeer(nodeId uint64) (*config.RaftNode, error)
	IsNodeRegistered(nodeId uint64) (bool, error)
	RefreshRegistration(ctx context.Context) error
	UnregisterNode() error
	UnregisterNodeById(nodeId uint64) error
	Close() error
}

type etcdService struct {
	client         *clientv3.Client
	lease          clientv3.LeaseID
	leaseKeepAlive <-chan *clientv3.LeaseKeepAliveResponse
	config         config.Scheduler0Config
	logger         hclog.Logger
	nodeId         uint64
	nodeInfo       config.RaftNode
	keyPrefix      string
	ttl            int64
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewEtcdService(ctx context.Context, logger hclog.Logger, scheduler0Config config.Scheduler0Config) (EtcdService, error) {
	configs := scheduler0Config.GetConfigurations()

	// Check if etcd is configured
	if len(configs.EtcdEndpoints) == 0 {
		return nil, fmt.Errorf("etcd endpoints not configured")
	}

	etcdLogger := logger.Named("etcd-service")

	etcdLogger.Info("etcd endpoints", "endpoints", configs.EtcdEndpoints)
	etcdLogger.Info("etcd key prefix", "keyPrefix", configs.EtcdKeyPrefix)
	etcdLogger.Info("etcd ttl", "ttl", configs.EtcdTTL)

	// Create etcd client config
	clientConfig := clientv3.Config{
		Endpoints:   configs.EtcdEndpoints,
		DialTimeout: 5 * time.Second,
	}

	// Create etcd client
	client, err := clientv3.New(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	// Set key prefix and TTL
	keyPrefix := configs.EtcdKeyPrefix
	if keyPrefix == "" {
		keyPrefix = "/scheduler0/nodes"
	}
	ttl := configs.EtcdTTL
	if ttl == 0 {
		ttl = 30
	}

	serviceCtx, cancel := context.WithCancel(ctx)

	service := &etcdService{
		client:    client,
		config:    scheduler0Config,
		logger:    etcdLogger,
		keyPrefix: keyPrefix,
		ttl:       ttl,
		ctx:       serviceCtx,
		cancel:    cancel,
	}

	etcdLogger.Info("etcd service initialized", "endpoints", configs.EtcdEndpoints, "keyPrefix", keyPrefix, "ttl", ttl)

	return service, nil
}

func (e *etcdService) RegisterNode(nodeId uint64, nodeInfo config.RaftNode) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logger.Info("registering node", "nodeId", nodeId, "nodeInfo", nodeInfo)

	e.nodeId = nodeId
	e.nodeInfo = nodeInfo

	// Check if node is already registered
	key := e.getNodeKey(nodeId)
	resp, err := e.client.Get(e.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check existing registration: %w", err)
	}

	// If node exists, check if lease is still valid
	if len(resp.Kvs) > 0 {
		// Check if the key has a valid lease
		leaseId := clientv3.LeaseID(resp.Kvs[0].Lease)
		if leaseId != 0 {
			// Check if lease is still alive
			ttlResp, err := e.client.TimeToLive(e.ctx, leaseId)
			if err == nil && ttlResp.TTL > 0 {
				e.logger.Warn("node already registered with valid lease", "nodeId", nodeId, "ttl", ttlResp.TTL)
				// Previous instance might still be alive, but we'll proceed anyway
				// In production, you might want to wait or fail here
			}
		}
	}

	// Create or get lease
	lease, err := e.client.Grant(e.ctx, e.ttl)
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}
	e.lease = lease.ID

	// Serialize node info
	nodeData, err := json.Marshal(nodeInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal node info: %w", err)
	}

	// Put key with lease
	_, err = e.client.Put(e.ctx, key, string(nodeData), clientv3.WithLease(e.lease))
	if err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// Start keep-alive
	keepAliveCh, err := e.client.KeepAlive(e.ctx, e.lease)
	if err != nil {
		return fmt.Errorf("failed to start keep-alive: %w", err)
	}
	e.leaseKeepAlive = keepAliveCh

	// Start background goroutine to handle keep-alive responses
	go e.handleKeepAlive()

	e.logger.Info("node registered in etcd", "nodeId", nodeId, "key", key)

	return nil
}

func (e *etcdService) handleKeepAlive() {
	for {
		select {
		case resp, ok := <-e.leaseKeepAlive:
			if !ok {
				e.logger.Warn("keep-alive channel closed, attempting to refresh lease")
				// Try to refresh registration
				if err := e.RefreshRegistration(e.ctx); err != nil {
					e.logger.Error("failed to refresh registration", "error", err)

					if strings.Contains(err.Error(), "requested lease not found") {
						e.logger.Error("requested lease not found, refreshing registration")
						e.client.Revoke(e.ctx, e.lease)
						err = e.RegisterNode(e.nodeId, e.nodeInfo)
						if err != nil {
							e.logger.Error("failed to re-register node", "error", err)
						}
					}
				}

				return
			}
			if resp != nil {
				e.logger.Debug("lease keep-alive successful", "ttl", resp.TTL)
			}
		case <-e.ctx.Done():
			return
		}
	}
}

func (e *etcdService) RefreshRegistration(ctx context.Context) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.logger.Info("refreshing registration", "lease", e.lease)

	if e.lease == 0 {
		return fmt.Errorf("no active lease to refresh")
	}

	// Keep-alive the lease
	_, err := e.client.KeepAliveOnce(ctx, e.lease)
	if err != nil {
		return fmt.Errorf("failed to refresh lease: %w", err)
	}

	return nil
}

func (e *etcdService) GetPeers(ctx context.Context) ([]config.RaftNode, error) {
	// Guard against nil context; fall back to service context used by the client.
	if ctx == nil {
		ctx = e.ctx
	}

	e.logger.Info("getting peers", "keyPrefix", e.keyPrefix)

	keyPrefix := e.keyPrefix + "/"
	resp, err := e.client.Get(ctx, keyPrefix, clientv3.WithPrefix())

	if err != nil {
		return nil, fmt.Errorf("failed to get peers: %w", err)
	}

	peers := make([]config.RaftNode, 0)
	for _, kv := range resp.Kvs {
		// Extract nodeId from key (format: /scheduler0/nodes/{nodeId})
		key := string(kv.Key)
		parts := strings.Split(key, "/")
		if len(parts) < 1 {
			continue
		}

		// Check if lease is still valid
		leaseId := clientv3.LeaseID(kv.Lease)
		if leaseId != 0 {
			ttlResp, err := e.client.TimeToLive(ctx, leaseId)
			if err != nil || ttlResp.TTL <= 0 {
				// Lease expired, skip this peer
				continue
			}
		}

		// Unmarshal node info
		var nodeInfo config.RaftNode
		if err := json.Unmarshal(kv.Value, &nodeInfo); err != nil {
			e.logger.Warn("failed to unmarshal node info", "key", key, "error", err)
			continue
		}

		peers = append(peers, nodeInfo)
	}

	return peers, nil
}

func (e *etcdService) GetPeer(nodeId uint64) (*config.RaftNode, error) {
	e.logger.Info("getting peer", "nodeId", nodeId)

	key := e.getNodeKey(nodeId)
	resp, err := e.client.Get(e.ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("peer not found: nodeId=%d", nodeId)
	}

	// Check if lease is still valid
	leaseId := clientv3.LeaseID(resp.Kvs[0].Lease)
	if leaseId != 0 {
		ttlResp, err := e.client.TimeToLive(e.ctx, leaseId)
		if err != nil || ttlResp.TTL <= 0 {
			return nil, fmt.Errorf("peer lease expired: nodeId=%d", nodeId)
		}
	}

	var nodeInfo config.RaftNode
	if err := json.Unmarshal(resp.Kvs[0].Value, &nodeInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node info: %w", err)
	}

	return &nodeInfo, nil
}

func (e *etcdService) IsNodeRegistered(nodeId uint64) (bool, error) {
	e.logger.Info("checking if node is registered", "nodeId", nodeId)

	key := e.getNodeKey(nodeId)
	resp, err := e.client.Get(e.ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to check node registration: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return false, nil
	}

	// Check if lease is still valid
	leaseId := clientv3.LeaseID(resp.Kvs[0].Lease)
	if leaseId != 0 {
		ttlResp, err := e.client.TimeToLive(e.ctx, leaseId)
		if err != nil || ttlResp.TTL <= 0 {
			return false, nil // Lease expired, node is not registered
		}
	}

	return true, nil
}

func (e *etcdService) WatchPeers(ctx context.Context) (<-chan []config.RaftNode, error) {
	e.logger.Info("watching peers", "keyPrefix", e.keyPrefix)

	peerCh := make(chan []config.RaftNode, 10)
	keyPrefix := e.keyPrefix + "/"

	// Initial peer list
	peers, err := e.GetPeers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get initial peers: %w", err)
	}
	peerCh <- peers

	// Start watching
	watchCh := e.client.Watch(ctx, keyPrefix, clientv3.WithPrefix())

	go func() {
		defer close(peerCh)

		for {
			select {
			case watchResp, ok := <-watchCh:
				if !ok {
					e.logger.Warn("watch channel closed")
					return
				}

				if watchResp.Err() != nil {
					e.logger.Error("watch error", "error", watchResp.Err())
					continue
				}

				// Get updated peer list
				peers, err := e.GetPeers(ctx)
				if err != nil {
					e.logger.Error("failed to get peers after watch event", "error", err)
					continue
				}

				select {
				case peerCh <- peers:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return peerCh, nil
}

func (e *etcdService) UnregisterNode() error {
	e.logger.Info("unregistering node", "nodeId", e.nodeId)

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.lease == 0 {
		return nil // Nothing to unregister
	}

	// Revoke lease to immediately remove registration
	_, err := e.client.Revoke(e.ctx, e.lease)
	if err != nil {
		return fmt.Errorf("failed to revoke lease: %w", err)
	}

	e.logger.Info("node unregistered from etcd", "nodeId", e.nodeId)

	return nil
}

// UnregisterNodeById removes a node from etcd by its ID. This is used by the leader
// to remove other nodes from the cluster.
func (e *etcdService) UnregisterNodeById(nodeId uint64) error {
	e.logger.Info("unregistering node by ID", "nodeId", nodeId)

	key := e.getNodeKey(nodeId)

	// First, try to get the key to find its lease
	resp, err := e.client.Get(e.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to get node key: %w", err)
	}

	if len(resp.Kvs) == 0 {
		// Node doesn't exist in etcd, nothing to do
		e.logger.Info("node not found in etcd", "nodeId", nodeId)
		return nil
	}

	// Try to revoke the lease if it exists
	leaseId := clientv3.LeaseID(resp.Kvs[0].Lease)
	if leaseId != 0 {
		_, err := e.client.Revoke(e.ctx, leaseId)
		if err != nil {
			// If revoking lease fails, try to delete the key directly
			e.logger.Warn("failed to revoke lease, deleting key directly", "nodeId", nodeId, "error", err)
			_, delErr := e.client.Delete(e.ctx, key)
			if delErr != nil {
				return fmt.Errorf("failed to revoke lease and delete key: revoke=%w, delete=%w", err, delErr)
			}
		}
	} else {
		// No lease, just delete the key
		_, err := e.client.Delete(e.ctx, key)
		if err != nil {
			return fmt.Errorf("failed to delete node key: %w", err)
		}
	}

	e.logger.Info("node unregistered from etcd", "nodeId", nodeId)
	return nil
}

func (e *etcdService) Close() error {
	e.logger.Info("closing etcd service")

	// Unregister node
	if err := e.UnregisterNode(); err != nil {
		e.logger.Error("failed to unregister node on close", "error", err)
	}

	// Cancel context
	e.cancel()

	// Close etcd client
	if err := e.client.Close(); err != nil {
		return fmt.Errorf("failed to close etcd client: %w", err)
	}

	e.logger.Info("etcd service closed")

	return nil
}

func (e *etcdService) getNodeKey(nodeId uint64) string {
	return fmt.Sprintf("%s/%d", e.keyPrefix, nodeId)
}
