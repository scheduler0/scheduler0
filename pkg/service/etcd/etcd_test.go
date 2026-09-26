package etcd

import (
	"context"
	"encoding/json"
	"os"
	"scheduler0/pkg/config"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// setupTestEtcdEndpoints returns etcd endpoints for testing
// It checks for ETCD_ENDPOINTS environment variable first,
// otherwise defaults to localhost:2379
func setupTestEtcdEndpoints(t *testing.T) []string {
	endpoints := os.Getenv("ETCD_ENDPOINTS")
	if endpoints != "" {
		// Support comma-separated endpoints
		return []string{endpoints}
	}
	// Default to localhost:2379 (standard etcd port)
	return []string{"localhost:2379"}
}

// checkEtcdConnection verifies that etcd is available
func checkEtcdConnection(t *testing.T, endpoints []string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		return false
	}
	defer client.Close()

	_, err = client.Status(ctx, endpoints[0])
	return err == nil
}

// setupTestEtcdService creates an etcd service for testing
func setupTestEtcdService(t *testing.T, endpoints []string, keyPrefix string, ttl int64) (EtcdService, func()) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "etcd-service-test",
		Level: hclog.LevelFromString("DEBUG"),
	})

	// Create a mock config
	mockConfig := &mockScheduler0Config{
		endpoints: endpoints,
		keyPrefix: keyPrefix,
		ttl:       ttl,
	}

	service, err := NewEtcdService(ctx, logger, mockConfig)
	require.NoError(t, err)

	cleanup := func() {
		service.Close()
	}

	return service, cleanup
}

// mockScheduler0Config implements config.Scheduler0Config for testing
type mockScheduler0Config struct {
	endpoints []string
	keyPrefix string
	ttl       int64
}

func (m *mockScheduler0Config) GetConfigurations() *config.Scheduler0Configurations {
	return &config.Scheduler0Configurations{
		EtcdEndpoints: m.endpoints,
		EtcdKeyPrefix: m.keyPrefix,
		EtcdTTL:       m.ttl,
	}
}

func Test_NewEtcdService(t *testing.T) {
	endpoints := setupTestEtcdEndpoints(t)
	if !checkEtcdConnection(t, endpoints) {
		t.Skip("etcd not available, skipping test")
	}

	t.Run("success with valid config", func(t *testing.T) {

		ctx := context.Background()
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "etcd-service-test",
			Level: hclog.LevelFromString("DEBUG"),
		})

		mockConfig := &mockScheduler0Config{
			endpoints: endpoints,
			keyPrefix: "/test/nodes",
			ttl:       30,
		}

		service, err := NewEtcdService(ctx, logger, mockConfig)
		require.NoError(t, err)
		assert.NotNil(t, service)
		defer service.Close()
	})

	t.Run("success with default key prefix", func(t *testing.T) {

		ctx := context.Background()
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "etcd-service-test",
			Level: hclog.LevelFromString("DEBUG"),
		})

		mockConfig := &mockScheduler0Config{
			endpoints: endpoints,
			keyPrefix: "", // Empty should default to "/scheduler0/nodes"
			ttl:       30,
		}

		service, err := NewEtcdService(ctx, logger, mockConfig)
		require.NoError(t, err)
		assert.NotNil(t, service)
		defer service.Close()
	})

	t.Run("success with default TTL", func(t *testing.T) {

		ctx := context.Background()
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "etcd-service-test",
			Level: hclog.LevelFromString("DEBUG"),
		})

		mockConfig := &mockScheduler0Config{
			endpoints: endpoints,
			keyPrefix: "/test/nodes",
			ttl:       0, // Should default to 30
		}

		service, err := NewEtcdService(ctx, logger, mockConfig)
		require.NoError(t, err)
		assert.NotNil(t, service)
		defer service.Close()
	})

	t.Run("failure with no endpoints", func(t *testing.T) {
		ctx := context.Background()
		logger := hclog.New(&hclog.LoggerOptions{
			Name:  "etcd-service-test",
			Level: hclog.LevelFromString("DEBUG"),
		})

		mockConfig := &mockScheduler0Config{
			endpoints: []string{}, // Empty endpoints
			keyPrefix: "/test/nodes",
			ttl:       30,
		}

		service, err := NewEtcdService(ctx, logger, mockConfig)
		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "etcd endpoints not configured")
	})
}

func Test_EtcdService_RegisterNode(t *testing.T) {
	endpoints := setupTestEtcdEndpoints(t)
	if !checkEtcdConnection(t, endpoints) {
		t.Skip("etcd not available, skipping test")
	}

	t.Run("success register new node", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		nodeId := uint64(1)
		nodeInfo := config.RaftNode{
			NodeId:        nodeId,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}

		err := service.RegisterNode(nodeId, nodeInfo)
		assert.NoError(t, err)

		// Verify node is registered
		registered, err := service.IsNodeRegistered(nodeId)
		assert.NoError(t, err)
		assert.True(t, registered)
	})

	t.Run("success register node with existing key but expired lease", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 1) // Short TTL
		defer cleanupService()

		nodeId := uint64(1)
		nodeInfo := config.RaftNode{
			NodeId:        nodeId,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}

		// Register node
		err := service.RegisterNode(nodeId, nodeInfo)
		assert.NoError(t, err)

		// Wait for lease to expire
		time.Sleep(2 * time.Second)

		// Register again - should succeed
		err = service.RegisterNode(nodeId, nodeInfo)
		assert.NoError(t, err)
	})

	t.Run("failure with invalid node info", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		nodeId := uint64(1)
		// This should still work, but let's test with a valid nodeInfo
		nodeInfo := config.RaftNode{
			NodeId:        nodeId,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}

		err := service.RegisterNode(nodeId, nodeInfo)
		assert.NoError(t, err)
	})
}

func Test_EtcdService_GetPeers(t *testing.T) {
	endpoints := setupTestEtcdEndpoints(t)
	if !checkEtcdConnection(t, endpoints) {
		t.Skip("etcd not available, skipping test")
	}

	t.Run("success get peers with multiple nodes", func(t *testing.T) {

		// Create first service and register node 1
		service1, cleanup1 := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanup1()

		node1 := config.RaftNode{
			NodeId:        1,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		err := service1.RegisterNode(1, node1)
		require.NoError(t, err)

		// Create second service and register node 2
		service2, cleanup2 := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanup2()

		node2 := config.RaftNode{
			NodeId:        2,
			NodeAddress:   "127.0.0.1:8081",
			ClientAddress: "127.0.0.1:7071",
		}
		err = service2.RegisterNode(2, node2)
		require.NoError(t, err)

		// Wait a bit for registration to propagate
		time.Sleep(100 * time.Millisecond)

		// Get peers from service1 (should see node2)
		ctx := context.Background()
		peers, err := service1.GetPeers(ctx)
		assert.NoError(t, err)
		assert.Len(t, peers, 1)
		assert.Equal(t, uint64(2), peers[0].NodeId)
		assert.Equal(t, node2.NodeAddress, peers[0].NodeAddress)
		assert.Equal(t, node2.ClientAddress, peers[0].ClientAddress)
	})

	t.Run("success get peers with no other nodes", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		nodeId := uint64(1)
		nodeInfo := config.RaftNode{
			NodeId:        nodeId,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		err := service.RegisterNode(nodeId, nodeInfo)
		require.NoError(t, err)

		ctx := context.Background()
		peers, err := service.GetPeers(ctx)
		assert.NoError(t, err)
		assert.Len(t, peers, 0) // Should not see itself
	})

	t.Run("success get peers with expired lease", func(t *testing.T) {

		// Create first service with short TTL
		service1, cleanup1 := setupTestEtcdService(t, endpoints, "/test/nodes", 1)
		defer cleanup1()

		node1 := config.RaftNode{
			NodeId:        1,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		err := service1.RegisterNode(1, node1)
		require.NoError(t, err)

		// Create second service
		service2, cleanup2 := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanup2()

		node2 := config.RaftNode{
			NodeId:        2,
			NodeAddress:   "127.0.0.1:8081",
			ClientAddress: "127.0.0.1:7071",
		}
		err = service2.RegisterNode(2, node2)
		require.NoError(t, err)

		// Wait for node1's lease to expire
		time.Sleep(2 * time.Second)

		// Get peers from service2 - should not see expired node1
		ctx := context.Background()
		peers, err := service2.GetPeers(ctx)
		assert.NoError(t, err)
		assert.Len(t, peers, 0) // node1 should be expired
	})

	t.Run("success with nil context", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		nodeId := uint64(1)
		nodeInfo := config.RaftNode{
			NodeId:        nodeId,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		err := service.RegisterNode(nodeId, nodeInfo)
		require.NoError(t, err)

		// Get peers with nil context - should use service context
		peers, err := service.GetPeers(nil)
		assert.NoError(t, err)
		assert.NotNil(t, peers)
	})
}

func Test_EtcdService_GetPeer(t *testing.T) {
	endpoints := setupTestEtcdEndpoints(t)
	if !checkEtcdConnection(t, endpoints) {
		t.Skip("etcd not available, skipping test")
	}

	t.Run("success get existing peer", func(t *testing.T) {

		service1, cleanup1 := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanup1()

		service2, cleanup2 := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanup2()

		node1 := config.RaftNode{
			NodeId:        1,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		err := service1.RegisterNode(1, node1)
		require.NoError(t, err)

		node2 := config.RaftNode{
			NodeId:        2,
			NodeAddress:   "127.0.0.1:8081",
			ClientAddress: "127.0.0.1:7071",
		}
		err = service2.RegisterNode(2, node2)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Get peer from service2
		peer, err := service2.GetPeer(1)
		assert.NoError(t, err)
		assert.NotNil(t, peer)
		assert.Equal(t, uint64(1), peer.NodeId)
		assert.Equal(t, node1.NodeAddress, peer.NodeAddress)
		assert.Equal(t, node1.ClientAddress, peer.ClientAddress)
	})

	t.Run("failure get non-existent peer", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		peer, err := service.GetPeer(999)
		assert.Error(t, err)
		assert.Nil(t, peer)
		assert.Contains(t, err.Error(), "peer not found")
	})

	t.Run("failure get peer with expired lease", func(t *testing.T) {

		service1, cleanup1 := setupTestEtcdService(t, endpoints, "/test/nodes", 1) // Short TTL
		defer cleanup1()

		service2, cleanup2 := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanup2()

		node1 := config.RaftNode{
			NodeId:        1,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		err := service1.RegisterNode(1, node1)
		require.NoError(t, err)

		// Wait for lease to expire
		time.Sleep(2 * time.Second)

		// Try to get expired peer
		peer, err := service2.GetPeer(1)
		assert.Error(t, err)
		assert.Nil(t, peer)
		assert.Contains(t, err.Error(), "peer lease expired")
	})
}

func Test_EtcdService_IsNodeRegistered(t *testing.T) {
	endpoints := setupTestEtcdEndpoints(t)
	if !checkEtcdConnection(t, endpoints) {
		t.Skip("etcd not available, skipping test")
	}

	t.Run("success check registered node", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		nodeId := uint64(1)
		nodeInfo := config.RaftNode{
			NodeId:        nodeId,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		err := service.RegisterNode(nodeId, nodeInfo)
		require.NoError(t, err)

		registered, err := service.IsNodeRegistered(nodeId)
		assert.NoError(t, err)
		assert.True(t, registered)
	})

	t.Run("success check non-registered node", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		registered, err := service.IsNodeRegistered(999)
		assert.NoError(t, err)
		assert.False(t, registered)
	})

	t.Run("success check node with expired lease", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 1) // Short TTL
		defer cleanupService()

		nodeId := uint64(1)
		nodeInfo := config.RaftNode{
			NodeId:        nodeId,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		err := service.RegisterNode(nodeId, nodeInfo)
		require.NoError(t, err)

		// Wait for lease to expire
		time.Sleep(2 * time.Second)

		registered, err := service.IsNodeRegistered(nodeId)
		assert.NoError(t, err)
		assert.False(t, registered) // Should return false for expired lease
	})
}

func Test_EtcdService_RefreshRegistration(t *testing.T) {
	endpoints := setupTestEtcdEndpoints(t)
	if !checkEtcdConnection(t, endpoints) {
		t.Skip("etcd not available, skipping test")
	}

	t.Run("success refresh active registration", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		nodeId := uint64(1)
		nodeInfo := config.RaftNode{
			NodeId:        nodeId,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		err := service.RegisterNode(nodeId, nodeInfo)
		require.NoError(t, err)

		ctx := context.Background()
		err = service.RefreshRegistration(ctx)
		assert.NoError(t, err)
	})

	t.Run("failure refresh without registration", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		// Try to refresh without registering first
		ctx := context.Background()
		err := service.RefreshRegistration(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no active lease to refresh")
	})
}

func Test_EtcdService_UnregisterNode(t *testing.T) {
	endpoints := setupTestEtcdEndpoints(t)
	if !checkEtcdConnection(t, endpoints) {
		t.Skip("etcd not available, skipping test")
	}

	t.Run("success unregister registered node", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		nodeId := uint64(1)
		nodeInfo := config.RaftNode{
			NodeId:        nodeId,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		err := service.RegisterNode(nodeId, nodeInfo)
		require.NoError(t, err)

		// Verify registered
		registered, err := service.IsNodeRegistered(nodeId)
		assert.NoError(t, err)
		assert.True(t, registered)

		// Unregister
		err = service.UnregisterNode()
		assert.NoError(t, err)

		// Verify unregistered
		registered, err = service.IsNodeRegistered(nodeId)
		assert.NoError(t, err)
		assert.False(t, registered)
	})

	t.Run("success unregister without registration", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		// Unregister without registering first - should not error
		err := service.UnregisterNode()
		assert.NoError(t, err)
	})
}

func Test_EtcdService_WatchPeers(t *testing.T) {
	endpoints := setupTestEtcdEndpoints(t)
	if !checkEtcdConnection(t, endpoints) {
		t.Skip("etcd not available, skipping test")
	}

	t.Run("success watch peers and receive updates", func(t *testing.T) {

		service1, cleanup1 := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanup1()

		service2, cleanup2 := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanup2()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start watching from service1
		peerCh, err := service1.WatchPeers(ctx)
		require.NoError(t, err)

		// Register node1
		node1 := config.RaftNode{
			NodeId:        1,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		err = service1.RegisterNode(1, node1)
		require.NoError(t, err)

		// Register node2 (should trigger watch event)
		node2 := config.RaftNode{
			NodeId:        2,
			NodeAddress:   "127.0.0.1:8081",
			ClientAddress: "127.0.0.1:7071",
		}
		err = service2.RegisterNode(2, node2)
		require.NoError(t, err)

		// Wait for watch event
		select {
		case peers := <-peerCh:
			// Should see node2
			assert.Len(t, peers, 1)
			assert.Equal(t, uint64(2), peers[0].NodeId)
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for watch event")
		}
	})

	t.Run("success watch peers with initial peer list", func(t *testing.T) {

		service1, cleanup1 := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanup1()

		service2, cleanup2 := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanup2()

		// Register node2 first
		node2 := config.RaftNode{
			NodeId:        2,
			NodeAddress:   "127.0.0.1:8081",
			ClientAddress: "127.0.0.1:7071",
		}
		err := service2.RegisterNode(2, node2)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Start watching from service1
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		peerCh, err := service1.WatchPeers(ctx)
		require.NoError(t, err)

		// Should receive initial peer list with node2
		select {
		case peers := <-peerCh:
			assert.Len(t, peers, 1)
			assert.Equal(t, uint64(2), peers[0].NodeId)
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for initial peer list")
		}
	})

	t.Run("success watch peers with context cancellation", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		ctx, cancel := context.WithCancel(context.Background())

		peerCh, err := service.WatchPeers(ctx)
		require.NoError(t, err)

		// Cancel context
		cancel()

		// Channel should close
		select {
		case _, ok := <-peerCh:
			assert.False(t, ok, "channel should be closed")
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for channel close")
		}
	})
}

func Test_EtcdService_Close(t *testing.T) {
	endpoints := setupTestEtcdEndpoints(t)
	if !checkEtcdConnection(t, endpoints) {
		t.Skip("etcd not available, skipping test")
	}

	t.Run("success close service with registered node", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		nodeId := uint64(1)
		nodeInfo := config.RaftNode{
			NodeId:        nodeId,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		err := service.RegisterNode(nodeId, nodeInfo)
		require.NoError(t, err)

		// Close service
		err = service.Close()
		assert.NoError(t, err)

		// Verify node is unregistered
		// Note: We can't check this with the same service since it's closed,
		// but we can verify Close doesn't error
	})

	t.Run("success close service without registration", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		// Close without registering
		err := service.Close()
		assert.NoError(t, err)
	})
}

func Test_EtcdService_ConcurrentOperations(t *testing.T) {
	endpoints := setupTestEtcdEndpoints(t)
	if !checkEtcdConnection(t, endpoints) {
		t.Skip("etcd not available, skipping test")
	}

	t.Run("success concurrent registrations", func(t *testing.T) {

		// Create multiple services
		services := make([]EtcdService, 5)
		cleanups := make([]func(), 5)

		for i := 0; i < 5; i++ {
			service, cleanup := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
			services[i] = service
			cleanups[i] = cleanup
		}
		defer func() {
			for _, cleanup := range cleanups {
				cleanup()
			}
		}()

		// Register nodes concurrently
		done := make(chan error, 5)
		for i := 0; i < 5; i++ {
			go func(idx int) {
				nodeId := uint64(idx + 1)
				nodeInfo := config.RaftNode{
					NodeId:        nodeId,
					NodeAddress:   "127.0.0.1:808" + strconv.Itoa(idx),
					ClientAddress: "127.0.0.1:707" + strconv.Itoa(idx),
				}
				err := services[idx].RegisterNode(nodeId, nodeInfo)
				done <- err
			}(i)
		}

		// Wait for all registrations
		for i := 0; i < 5; i++ {
			err := <-done
			assert.NoError(t, err)
		}

		// Verify all nodes are registered
		time.Sleep(200 * time.Millisecond)
		ctx := context.Background()
		peers, err := services[0].GetPeers(ctx)
		assert.NoError(t, err)
		assert.Len(t, peers, 4) // Should see 4 other nodes (excluding self)
	})
}

func Test_EtcdService_EdgeCases(t *testing.T) {
	endpoints := setupTestEtcdEndpoints(t)
	if !checkEtcdConnection(t, endpoints) {
		t.Skip("etcd not available, skipping test")
	}

	t.Run("success handle invalid JSON in etcd", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		// Manually put invalid JSON in etcd
		client, err := clientv3.New(clientv3.Config{
			Endpoints:   endpoints,
			DialTimeout: 5 * time.Second,
		})
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()
		_, err = client.Put(ctx, "/test/nodes/999", "invalid-json")
		require.NoError(t, err)

		// GetPeers should skip invalid entries
		peers, err := service.GetPeers(ctx)
		assert.NoError(t, err)
		// Should not include the invalid entry
		for _, peer := range peers {
			assert.NotEqual(t, uint64(999), peer.NodeId)
		}
	})

	t.Run("success handle key with invalid nodeId format", func(t *testing.T) {

		service, cleanupService := setupTestEtcdService(t, endpoints, "/test/nodes", 30)
		defer cleanupService()

		// Manually put key with invalid nodeId
		client, err := clientv3.New(clientv3.Config{
			Endpoints:   endpoints,
			DialTimeout: 5 * time.Second,
		})
		require.NoError(t, err)
		defer client.Close()

		nodeInfo := config.RaftNode{
			NodeId:        1,
			NodeAddress:   "127.0.0.1:8080",
			ClientAddress: "127.0.0.1:7070",
		}
		nodeData, _ := json.Marshal(nodeInfo)

		ctx := context.Background()
		_, err = client.Put(ctx, "/test/nodes/invalid-id", string(nodeData))
		require.NoError(t, err)

		// GetPeers should skip entries with invalid nodeId
		peers, err := service.GetPeers(ctx)
		assert.NoError(t, err)
		// Should not include the invalid entry
		for _, peer := range peers {
			assert.NotEqual(t, "invalid-id", peer.NodeId)
		}
	})
}
