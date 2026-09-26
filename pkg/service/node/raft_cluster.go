package node

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/fsm"
	"scheduler0/pkg/utils"
	"strconv"

	"github.com/hashicorp/raft"
)

type RaftClusterManager interface {
	Start()
	RemoveSelfFromCluster(ctx context.Context) error
	AddSelfToCluster(ctx context.Context) error
	ForceRebuildCluster(ctx context.Context, seedNodeId uint64) error
	ResetRaftState(ctx context.Context) error
	ReconcileRaftMembershipWithPeers(peers []config.RaftNode)
	AuthRaftConfiguration() raft.Configuration
	GetRaftStats() map[string]string
	GetRaftLeaderWithId() (raft.ServerAddress, raft.ServerID)
	HandleRaftLeadershipChanges(isLeader bool)
	HandleRaftLeadershipChangesDebounced(isLeader bool)
	HandleRaftObserverChannelChanges(o raft.Observation)
	RemoveNode(ctx context.Context, nodeId uint64) error
	AddNode(ctx context.Context, nodeId uint64, nodeAddress string, clientAddress string) error
	PromoteNode(ctx context.Context, nodeId uint64) error
	DemoteNode(ctx context.Context, nodeId uint64) error
	TransferLeadership(ctx context.Context, targetNodeId *uint64) error
	ListNodes(ctx context.Context) ([]config.RaftNode, error)
}

type raftClusterManager struct {
	node *nodeService
}

func newRaftClusterManager(node *nodeService) *raftClusterManager {
	return &raftClusterManager{
		node: node,
	}
}

func (r *raftClusterManager) Start() {
	r.node.logger.Info("Staring Node Service")

	configs := r.node.scheduler0Config.GetConfigurations()

	// Register node in etcd FIRST (before recovery/bootstrap logic)
	// This is critical for ECS no-downtime deploys
	if r.node.etcdService != nil && len(configs.EtcdEndpoints) > 0 {
		// Use service discovery address for NodeAddress (used for both node-to-node and raft communications)
		nodeInfo := config.RaftNode{
			NodeId:        configs.NodeId,
			NodeAddress:   configs.NodeServiceDiscoveryAddress,
			ClientAddress: fmt.Sprintf("%s:%s", configs.ServiceDiscoveryHost, configs.ClientPort),
		}

		if err := r.node.etcdService.RegisterNode(configs.NodeId, nodeInfo); err != nil {
			r.node.logger.Error("failed to register node in etcd", "error", err)
			// Continue anyway - node may still be able to join cluster via raft
		} else {
			r.node.logger.Info("node registered in etcd", "nodeId", configs.NodeId)

			// Start watching peers from etcd
			go r.node.peerComm.WatchPeersFromEtcd()
		}
	}

	// Recovery logic
	if r.node.isExistingNode {
		r.node.logger.Info("discovered existing raft dir")
		r.node.scheduler0RaftStore.RecoverRaftState()
	}

	if configs.Bootstrap && !r.node.isExistingNode {

		if r.node.etcdService != nil && len(configs.EtcdEndpoints) > 0 {
			peers, err := r.node.etcdService.GetPeers(r.node.ctx)
			r.node.logger.Info("peers from etcd", "peers", peers)
			if err != nil {
				r.node.logger.Warn("failed to get peers from etcd, proceeding with bootstrap check", "error", err)
				// On error, proceed with bootstrap to be safe
				r.node.logger.Info("bootstrapping cluster due to etcd error")
				cfg := r.AuthRaftConfiguration()
				r.node.scheduler0RaftStore.BootstrapRaftClusterWithConfig(cfg)
			} else if len(peers) > 0 {
				// Peers exist in etcd, but check if there's actually a Raft cluster running
				// A non-bootstrap node might have registered in etcd before the bootstrap node started
				raftObj := r.node.scheduler0RaftStore.GetRaft()
				hasCluster := false
				if raftObj != nil {
					// Check if there's a leader (indicates an active cluster)
					leaderAddr, leaderID := raftObj.LeaderWithID()
					if leaderAddr != "" && leaderID != "" {
						hasCluster = true
						r.node.logger.Info("detected existing Raft cluster with leader", "leaderID", leaderID, "leaderAddr", leaderAddr)
					} else {
						// Check if there's a configuration (indicates cluster was bootstrapped)
						cfgFuture := raftObj.GetConfiguration()
						if cfgFuture.Error() == nil {
							cfg := cfgFuture.Configuration()
							if len(cfg.Servers) > 0 {
								hasCluster = true
								r.node.logger.Info("detected existing Raft cluster configuration", "serverCount", len(cfg.Servers))
							}
						}
					}
				}

				if hasCluster {
					r.node.logger.Info("other nodes exist in etcd and Raft cluster is active, joining cluster instead of bootstrapping", "peerCount", len(peers))
					// Skip bootstrap, will join existing cluster
				} else {
					r.node.logger.Info("other nodes exist in etcd but no active Raft cluster detected, bootstrapping cluster", "peerCount", len(peers))
					// No active cluster, bootstrap anyway (peers will join via etcd watch mechanism)
					cfg := r.AuthRaftConfiguration()
					r.node.scheduler0RaftStore.BootstrapRaftClusterWithConfig(cfg)
				}
			} else {
				// No other nodes, safe to bootstrap
				r.node.logger.Info("no other nodes in etcd, bootstrapping cluster")
				cfg := r.AuthRaftConfiguration()
				r.node.logger.Info("bootstrapping cluster with configuration", "configuration", cfg)
				r.node.scheduler0RaftStore.BootstrapRaftClusterWithConfig(cfg)
			}
		} else {
			r.node.logger.Info("no etcd configured, bootstrapping cluster")
			cfg := r.AuthRaftConfiguration()
			r.node.scheduler0RaftStore.BootstrapRaftClusterWithConfig(cfg)
		}

		if r.node.sqliteDbExists {
			r.node.logger.Info("sqlite db exists, creating snapshot from sqlite db")
			dataStore := r.node.scheduler0RaftStore.GetDataStore()
			snapshot := fsm.NewFSMSnapshot(dataStore)
			rCfg := r.node.scheduler0RaftStore.GetRaft().GetConfiguration().Configuration()
			sink, err := r.node.FileSnapShot.Create(1, 1, 1, rCfg, 1, r.node.TransportManager)
			if err != nil {
				r.node.logger.Error("failed to create snapshot sink from sqlite db on bootstrap", "error", err)
			} else {
				r.node.logger.Info("snapshot sink created from sqlite db on bootstrap")
			}
			if err := snapshot.Persist(sink); err != nil {
				r.node.logger.Error("failed to persist snapshot from sqlite db on bootstrap", "error", err)
			} else {
				r.node.logger.Info("snapshot persisted from sqlite db on bootstrap")
			}
			if err := sink.Close(); err != nil {
				r.node.logger.Error("failed to close snapshot sink from sqlite db on bootstrap", "error", err)
			} else {
				r.node.logger.Info("snapshot sink closed from sqlite db on bootstrap")
			}
		}
	}

	r.node.logger.Info("registering observer")

	myObserver := raft.NewObserver(r.node.peerObserverChannels, true, func(o *raft.Observation) bool {
		_, peerObservation := o.Data.(raft.PeerObservation)
		_, resumedHeartbeatObservation := o.Data.(raft.ResumedHeartbeatObservation)
		return peerObservation || resumedHeartbeatObservation
	})

	r.node.scheduler0RaftStore.RegisterObserver(myObserver)

	// Check initial leader state - LeaderCh() only emits on changes, not initial state
	// If node is already leader after recovery, we need to initialize acceptClientWrites
	raftObj := r.node.scheduler0RaftStore.GetRaft()
	if raftObj != nil && raftObj.State() == raft.Leader {
		r.node.logger.Info("node is already leader on startup, initializing leader state")
		// Handle leadership changes synchronously to ensure acceptClientWrites is set
		// before HTTP server starts accepting requests
		// For initial startup, call directly without debounce to ensure immediate initialization
		r.HandleRaftLeadershipChanges(true)
	}

	r.node.serviceState.BeginAcceptingClientRequest()

	go r.node.eventHandler.ListenOnInputQueues()
}

func (r *raftClusterManager) GetRaftStats() map[string]string {
	return r.node.scheduler0RaftStore.GetRaftStats()
}

func (r *raftClusterManager) GetRaftLeaderWithId() (raft.ServerAddress, raft.ServerID) {
	return r.node.scheduler0RaftStore.LeaderWithID()
}

// RemoveSelfFromCluster removes this node from Raft membership (leader-only) and unregisters it from etcd.
func (r *raftClusterManager) RemoveSelfFromCluster(ctx context.Context) error {
	configs := r.node.scheduler0Config.GetConfigurations()

	// Unregister from etcd first
	if r.node.etcdService != nil {
		if err := r.node.etcdService.UnregisterNode(); err != nil {
			r.node.logger.Error("failed to unregister node from etcd", "error", err)
			return fmt.Errorf("failed to unregister from etcd: %w", err)
		}
	}

	raftObj := r.node.scheduler0RaftStore.GetRaft()
	if raftObj == nil {
		return fmt.Errorf("raft not initialized")
	}

	if raftObj.State() != raft.Leader {
		return fmt.Errorf("node is not leader; cannot remove self")
	}

	serverID := raft.ServerID(strconv.FormatUint(configs.NodeId, 10))
	future := raftObj.RemoveServer(serverID, 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to remove self from raft: %w", err)
	}

	r.node.logger.Info("removed self from raft cluster and etcd", "nodeId", configs.NodeId)
	return nil
}

// AddSelfToCluster ensures this node is registered in etcd and added as a voter to the Raft cluster.
// Requires the node to be the current Raft leader to mutate membership.
func (r *raftClusterManager) AddSelfToCluster(ctx context.Context) error {
	configs := r.node.scheduler0Config.GetConfigurations()

	// Register (or refresh) in etcd
	if r.node.etcdService != nil {
		nodeInfo := config.RaftNode{
			NodeId:        configs.NodeId,
			NodeAddress:   configs.NodeServiceDiscoveryAddress,
			ClientAddress: fmt.Sprintf("%s:%s", configs.ServiceDiscoveryHost, configs.ClientPort),
		}
		if err := r.node.etcdService.RegisterNode(configs.NodeId, nodeInfo); err != nil {
			r.node.logger.Error("failed to register node in etcd", "error", err)
			return fmt.Errorf("failed to register in etcd: %w", err)
		}
	}

	raftObj := r.node.scheduler0RaftStore.GetRaft()
	if raftObj == nil {
		return fmt.Errorf("raft not initialized")
	}

	if raftObj.State() != raft.Leader {
		return fmt.Errorf("node is not leader; cannot add self")
	}

	serverID := raft.ServerID(strconv.FormatUint(configs.NodeId, 10))
	serverAddr := raft.ServerAddress(configs.NodeServiceDiscoveryAddress)

	// Check if already present
	currentCfg := raftObj.GetConfiguration()
	for _, srv := range currentCfg.Configuration().Servers {
		if srv.ID == serverID {
			// Already in membership; nothing to do
			r.node.logger.Info("node already part of raft cluster", "nodeId", configs.NodeId, "address", configs.NodeServiceDiscoveryAddress)
			return nil
		}
	}

	future := raftObj.AddVoter(serverID, serverAddr, 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to add self to raft: %w", err)
	}

	r.node.logger.Info("added self to raft cluster", "nodeId", configs.NodeId, "address", configs.NodeServiceDiscoveryAddress)
	return nil
}

// ForceRebuildCluster forces a rebuild of the Raft cluster by creating a snapshot from SQLite,
// wiping Raft state, and bootstrapping a new cluster. This should only be called on the seed node.
func (r *raftClusterManager) ForceRebuildCluster(ctx context.Context, seedNodeId uint64) error {
	configs := r.node.scheduler0Config.GetConfigurations()

	// Verify this is the seed node
	if configs.NodeId != seedNodeId {
		return fmt.Errorf("node %d is not the seed node %d; cannot force rebuild", configs.NodeId, seedNodeId)
	}

	r.node.logger.Info("starting force rebuild of Raft cluster", "nodeId", configs.NodeId, "seedNodeId", seedNodeId)

	raftObj := r.node.scheduler0RaftStore.GetRaft()
	if raftObj == nil {
		return fmt.Errorf("raft not initialized")
	}

	// Check if there's already a leader (optional safeguard, but we'll proceed anyway)
	if raftObj.State() == raft.Leader {
		r.node.logger.Warn("force rebuild called while node is leader; proceeding anyway")
	}

	// Step 1: Create snapshot from current SQLite before wiping state
	r.node.logger.Info("creating snapshot from SQLite before force rebuild")
	dataStore := r.node.scheduler0RaftStore.GetDataStore()
	if dataStore == nil {
		return fmt.Errorf("data store not available")
	}

	// Create snapshot from SQLite
	snapshot := fsm.NewFSMSnapshot(dataStore)

	// Build Raft configuration with only the seed node itself.
	// Other nodes will automatically join via etcd watch mechanism when they restart.
	r.node.logger.Info("bootstrapping with only seed node; other nodes will join automatically via etcd watch")
	servers := []raft.Server{
		{
			ID:       raft.ServerID(strconv.FormatUint(configs.NodeId, 10)),
			Suffrage: raft.Nonvoter,
			Address:  raft.ServerAddress(configs.NodeServiceDiscoveryAddress),
		},
	}
	bootstrapConfig := raft.Configuration{Servers: servers}

	// Step 2: Shutdown Raft cleanly
	r.node.logger.Info("shutting down Raft")
	if err := raftObj.Shutdown().Error(); err != nil {
		return fmt.Errorf("failed to shutdown raft: %w", err)
	}

	// Step 3: Clear existing snapshots before creating a new bootstrap snapshot.
	// Raft's BootstrapCluster only works on new clusters; existing snapshots/config
	// will cause "bootstrap only works on new clusters".
	dirPath := fmt.Sprintf("%v/%v", constants.RaftDir, configs.NodeId)
	snapshotDir := filepath.Join(dirPath, "snapshots")

	r.node.logger.Info("clearing existing raft snapshots", "snapshotDir", snapshotDir)
	if err := os.RemoveAll(snapshotDir); err != nil && !os.IsNotExist(err) {
		r.node.logger.Warn("failed to remove snapshot directory", "error", err)
	}

	// Step 4: Create snapshot and persist it to snapshot store
	// Use index=1, term=1 for bootstrap snapshot
	r.node.logger.Info("persisting snapshot to snapshot store")
	sink, err := r.node.FileSnapShot.Create(1, 1, 1, bootstrapConfig, 1, r.node.TransportManager)
	if err != nil {
		return fmt.Errorf("failed to create snapshot sink: %w", err)
	}

	if err := snapshot.Persist(sink); err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to persist snapshot: %w", err)
	}

	if err := sink.Close(); err != nil {
		return fmt.Errorf("failed to close snapshot sink: %w", err)
	}

	// Step 5: Wipe Raft state (logs and config store, but preserve snapshot store)
	r.node.logger.Info("wiping Raft logs and config store")

	// Close existing stores
	if r.node.LogDb != nil {
		if err := r.node.LogDb.Close(); err != nil {
			r.node.logger.Warn("failed to close log db", "error", err)
		}
	}
	if r.node.StoreDb != nil {
		if err := r.node.StoreDb.Close(); err != nil {
			r.node.logger.Warn("failed to close store db", "error", err)
		}
	}

	// Delete old database files to avoid file lock issues
	logFilePath := filepath.Join(dirPath, constants.RaftLog)
	stableFilePath := filepath.Join(dirPath, constants.RaftStableLog)

	if err := os.Remove(logFilePath); err != nil && !os.IsNotExist(err) {
		r.node.logger.Warn("failed to remove old log db file", "error", err)
	}
	if err := os.Remove(stableFilePath); err != nil && !os.IsNotExist(err) {
		r.node.logger.Warn("failed to remove old stable db file", "error", err)
	}

	// Reinitialize Raft components (this creates new empty logs/config stores)
	// Note: FileSnapshotStore will be recreated but will see our snapshot since it reads from filesystem
	r.node.logger.Info("reinitializing Raft components")
	ldb, stb, fss, tm, ln := utils.ConnectRaftLogsAndTransport(r.node.raftLn, r.node.scheduler0Config)
	r.node.LogDb = ldb
	r.node.StoreDb = stb
	r.node.FileSnapShot = fss
	if tm != nil {
		r.node.TransportManager = tm.(*raft.NetworkTransport)
	}
	_ = ln // Keep reference to listener

	// Update FSM store's internal store references
	r.node.scheduler0RaftStore.UpdateStores(ldb, stb, fss, tm)

	// Step 5: Reinitialize Raft with new stores
	r.node.logger.Info("reinitializing Raft instance")
	r.node.scheduler0RaftStore.InitRaft()

	// Step 6: Bootstrap new cluster with configuration from etcd
	r.node.logger.Info("bootstrapping new Raft cluster", "serverCount", len(servers))
	r.node.scheduler0RaftStore.BootstrapRaftClusterWithConfig(bootstrapConfig)

	nodeInfo := config.RaftNode{
		NodeId:        configs.NodeId,
		NodeAddress:   configs.NodeServiceDiscoveryAddress,
		ClientAddress: fmt.Sprintf("%s:%s", configs.ServiceDiscoveryHost, configs.ClientPort),
	}

	if err := r.node.etcdService.RegisterNode(configs.NodeId, nodeInfo); err != nil {
		r.node.logger.Error("failed to register node in etcd after force rebuild", "error", err)
		return fmt.Errorf("failed to register in etcd after force rebuild: %w", err)
	}

	r.node.logger.Info("force rebuild completed successfully", "nodeId", configs.NodeId)
	return nil
}

// ResetRaftState clears local Raft state (logs, stable store, snapshots) and
// exits the process to allow Fargate to restart the container with clean state.
// This is intended for non-seed nodes during remediation.
// The process will exit after successfully deleting files, allowing Fargate to
// spawn a new container that will start fresh and join the cluster via etcd.
func (r *raftClusterManager) ResetRaftState(ctx context.Context) error {
	configs := r.node.scheduler0Config.GetConfigurations()

	r.node.logger.Info("resetting local Raft state", "nodeId", configs.NodeId)

	raftObj := r.node.scheduler0RaftStore.GetRaft()
	if raftObj != nil {
		// Shutdown Raft cleanly before deleting files
		r.node.logger.Info("shutting down Raft before reset")
		if err := raftObj.Shutdown().Error(); err != nil {
			return fmt.Errorf("failed to shutdown raft before reset: %w", err)
		}
	}

	// Close existing stores
	if r.node.LogDb != nil {
		if err := r.node.LogDb.Close(); err != nil {
			r.node.logger.Warn("failed to close log db during reset", "error", err)
		}
	}
	if r.node.StoreDb != nil {
		if err := r.node.StoreDb.Close(); err != nil {
			r.node.logger.Warn("failed to close store db during reset", "error", err)
		}
	}

	// Clear FileSnapshotStore reference to allow file handles to be released
	r.node.FileSnapShot = nil

	// Clear Raft state by deleting individual files instead of the entire directory.
	// This avoids NFS lock issues with directory deletion while achieving the same result.
	dirPath := fmt.Sprintf("%v/%v", constants.RaftDir, configs.NodeId)
	logFilePath := filepath.Join(dirPath, constants.RaftLog)
	stableFilePath := filepath.Join(dirPath, constants.RaftStableLog)
	snapshotDir := filepath.Join(dirPath, "snapshots")

	r.node.logger.Info("clearing Raft state files", "dirPath", dirPath)

	// Delete individual files (simpler than directory deletion, avoids NFS locks)
	if err := os.Remove(logFilePath); err != nil && !os.IsNotExist(err) {
		r.node.logger.Warn("failed to remove log file during reset", "error", err)
		// Don't exit on error - return error instead
		return fmt.Errorf("failed to remove log file: %w", err)
	}
	r.node.logger.Info("removed log file", "path", logFilePath)

	if err := os.Remove(stableFilePath); err != nil && !os.IsNotExist(err) {
		r.node.logger.Warn("failed to remove stable file during reset", "error", err)
		// Don't exit on error - return error instead
		return fmt.Errorf("failed to remove stable file: %w", err)
	}
	r.node.logger.Info("removed stable file", "path", stableFilePath)

	// Clear snapshots directory (this is separate and usually works fine)
	if err := os.RemoveAll(snapshotDir); err != nil && !os.IsNotExist(err) {
		r.node.logger.Warn("failed to remove snapshot directory during reset", "error", err)
		// Don't exit on error - return error instead
		return fmt.Errorf("failed to remove snapshot directory: %w", err)
	}
	r.node.logger.Info("removed snapshot directory", "path", snapshotDir)

	// All files deleted successfully. Exit the process to allow Fargate to restart
	// the container with clean state. The new container will start fresh and join
	// the cluster via etcd watch mechanism.
	r.node.logger.Info("Raft reset complete, exiting for container restart", "nodeId", configs.NodeId)
	os.Exit(0)

	// This line should never be reached, but included for completeness
	return nil
}

// ReconcileRaftMembershipWithPeers ensures that the Raft cluster membership
// matches the set of peers discovered in etcd. This should only make changes
// when the local node is the Raft leader.
func (r *raftClusterManager) ReconcileRaftMembershipWithPeers(peers []config.RaftNode) {
	r.node.logger.Info("reconciling raft membership with peers", "peers", peers)
	raftObj := r.node.scheduler0RaftStore.GetRaft()
	if raftObj == nil {
		r.node.logger.Debug("reconcileRaftMembershipWithPeers skipped: raft not initialized yet")
		return
	}

	if raftObj.State() != raft.Leader {
		// Only the leader should perform membership changes.
		r.node.logger.Info("reconcileRaftMembershipWithPeers skipped: node is not leader")
		return
	}

	servers := r.node.scheduler0RaftStore.GetServersOnRaftCluster()
	existingByID := make(map[raft.ServerID]raft.Server, len(servers))
	for _, s := range servers {
		existingByID[s.ID] = s
	}

	configs := r.node.scheduler0Config.GetConfigurations()

	// Build a map of peer IDs from etcd for quick lookup
	peersByID := make(map[raft.ServerID]config.RaftNode)
	for _, peer := range peers {
		peerID := raft.ServerID(strconv.FormatUint(peer.NodeId, 10))
		peersByID[peerID] = peer
	}

	// Remove servers from Raft that are no longer in etcd (except self)
	for serverID, server := range existingByID {
		// Never remove self
		if serverID == raft.ServerID(strconv.FormatUint(configs.NodeId, 10)) {
			continue
		}

		// If server is not in etcd peers, remove it from Raft
		if _, existsInEtcd := peersByID[serverID]; !existsInEtcd {
			r.node.logger.Warn("removing raft server that is no longer in etcd",
				"serverId", serverID,
				"serverAddress", server.Address)

			// RemoveServer removes a server from the cluster
			future := raftObj.RemoveServer(serverID, 0, 0)
			if err := future.Error(); err != nil {
				r.node.logger.Error("failed to remove raft server that is no longer in etcd",
					"serverId", serverID,
					"serverAddress", server.Address,
					"error", err)
				continue
			}

			r.node.logger.Info("successfully removed raft server that is no longer in etcd",
				"serverId", serverID,
				"serverAddress", server.Address)
		}
	}

	// Add new peers from etcd that are not yet in Raft
	r.node.logger.Info("adding new raft peers from etcd", "peers", peers)
	for _, peer := range peers {
		peerID := raft.ServerID(strconv.FormatUint(peer.NodeId, 10))

		// Skip if this Raft ID is already part of the cluster.
		if _, ok := existingByID[peerID]; ok {
			continue
		}

		// Never attempt to add ourself here; self is added during bootstrap.
		if peer.NodeId == configs.NodeId {
			continue
		}

		r.node.logger.Info("adding new raft peer from etcd",
			"peerNodeId", peer.NodeId,
			"peerNodeAddress", peer.NodeAddress)

		future := raftObj.AddVoter(
			peerID,
			raft.ServerAddress(peer.NodeAddress),
			0, 0,
		)
		if err := future.Error(); err != nil {
			r.node.logger.Error("failed to add raft voter for peer discovered via etcd",
				"peerNodeId", peer.NodeId,
				"peerNodeAddress", peer.NodeAddress,
				"error", err)
			continue
		}

		r.node.logger.Info("successfully added raft voter for peer discovered via etcd",
			"peerNodeId", peer.NodeId,
			"peerNodeAddress", peer.NodeAddress)
	}
}

func (r *raftClusterManager) AuthRaftConfiguration() raft.Configuration {
	r.node.mtx.Lock()
	defer r.node.mtx.Unlock()

	configs := r.node.scheduler0Config.GetConfigurations()
	results := r.node.peerComm.AuthenticateWithPeersFromEtcd()
	servers := []raft.Server{
		{
			ID:       raft.ServerID(strconv.FormatUint(configs.NodeId, 10)),
			Suffrage: raft.Voter,
			Address:  raft.ServerAddress(configs.NodeServiceDiscoveryAddress),
		},
	}

	// Use peers from etcd
	peers := r.node.peerComm.GetPeers()

	for _, replica := range peers {
		if repStatus, ok := results[replica.NodeAddress]; ok && repStatus.IsAlive && repStatus.IsAuth {
			servers = append(servers, raft.Server{
				ID:       raft.ServerID(strconv.FormatUint(replica.NodeId, 10)),
				Suffrage: raft.Nonvoter,
				Address:  raft.ServerAddress(replica.NodeAddress),
			})
		}
	}

	cfg := raft.Configuration{
		Servers: servers,
	}
	return cfg
}

// HandleRaftLeadershipChangesDebounced is the debounced wrapper that stores the latest isLeader value
// and executes handleRaftLeadershipChanges with debounce to prevent rapid successive calls
func (r *raftClusterManager) HandleRaftLeadershipChangesDebounced(isLeader bool) {
	r.node.latestIsLeaderMtx.Lock()
	r.node.latestIsLeader = isLeader
	r.node.latestIsLeaderMtx.Unlock()

	// Debounce with 15 second delay (15000ms) to prevent rapid successive calls
	r.node.leadershipDebounce.Debounce(r.node.ctx, 15000, func() {
		r.node.latestIsLeaderMtx.Lock()
		latestValue := r.node.latestIsLeader
		r.node.latestIsLeaderMtx.Unlock()

		r.node.logger.Debug("executing debounced raft leadership changes", "isLeader", latestValue)
		r.HandleRaftLeadershipChanges(latestValue)
	})
}

func (r *raftClusterManager) HandleRaftLeadershipChanges(isLeader bool) {
	r.node.logger.Debug("handle raft leadership changes", "isLeader", isLeader)
	r.node.serviceState.StopAcceptingClientWriteRequest()
	servers := r.node.scheduler0RaftStore.GetServersOnRaftCluster()
	r.node.logger.Info("servers on raft cluster", "servers", servers)

	singleNodeMode := len(servers) == 1

	r.node.SingleNodeMode = singleNodeMode

	r.node.peerComm.StopAllJobsOnAllWorkerNodes()

	r.node.jobQueue.SetSingleNodeMode(singleNodeMode)
	r.node.jobQueue.SetNodeIsLeader(isLeader)
	r.node.jobQueue.SetNumberOfActiveNodes(uint64(len(servers)))

	r.node.jobExecutor.StopAll()
	r.node.jobExecutor.SetSingleNodeMode(singleNodeMode)
	r.node.jobExecutor.SetNodeIsLeader(isLeader)

	r.node.asyncTaskManager.SetSingleNodeMode(singleNodeMode)
	r.node.asyncTaskManager.SetNodeIsLeader(isLeader)

	r.node.jobProcessor.SetSingleNodeMode(singleNodeMode)
	r.node.jobProcessor.SetNodeIsLeader(isLeader)

	r.node.serviceState.ResetLocalQuotaAllocations()

	if isLeader {
		r.node.logger.Info("leader selected", "isLeader", isLeader)

		if !singleNodeMode {
			uncommittedLogs := r.node.jobExecutor.GetUncommittedLogs()
			r.node.logger.Info("uncommitted logs", "uncommittedLogs", len(uncommittedLogs))
			uncommittedAsyncTasks, err := r.node.asyncTaskManager.GetUnCommittedTasks()
			r.node.logger.Info("uncommitted async tasks", "uncommittedAsyncTasks", len(uncommittedAsyncTasks))
			if err != nil {
				log.Fatalln("failed to get uncommitted async tasks after leader selection", "error", err.Error())
			}
			if len(uncommittedLogs) > 0 {
				r.node.jobExecutionRepo.RaftInsertExecutionLogs(uncommittedLogs, r.node.scheduler0Config.GetConfigurations().NodeId)
			}

			if len(uncommittedAsyncTasks) > 0 {
				_, err := r.node.asyncTaskRepo.RaftBatchInsert(uncommittedAsyncTasks, r.node.scheduler0Config.GetConfigurations().NodeId)
				if err != nil {
					r.node.logger.Error("failed to insert uncommitted async tasks from", "raft-leader", "error", err)
				}
				r.node.eventHandler.HandleUncommittedAsyncTasks(uncommittedAsyncTasks)
			}

			r.node.logger.Info("starting synchronous fan-in from peers for leader initialization")
			r.node.peerComm.FanInLocalDataFromPeersSync()

			r.node.logger.Info("synchronous fan-in completed, starting periodic async fan-in")
			r.node.peerComm.FanInLocalDataFromPeers()

			r.node.logger.Info("calling beginAcceptingClientWriteRequest() after sync fan-in")
			r.node.jobProcessor.StartJobs()

			r.node.serviceState.BeginAcceptingClientWriteRequest()
		} else {
			r.node.logger.Debug("starting jobs on leader node")
			r.node.jobProcessor.StartJobs()
			r.node.serviceState.BeginAcceptingClientWriteRequest()
		}
	}
}

func (r *raftClusterManager) HandleRaftObserverChannelChanges(o raft.Observation) {
	peerObservation, isPeerObservation := o.Data.(raft.PeerObservation)
	resumedHeartbeatObservation, isResumedHeartbeatObservation := o.Data.(raft.ResumedHeartbeatObservation)

	if isPeerObservation && !peerObservation.Removed {
		r.node.logger.Debug("A new node joined the cluster", "nodeId", peerObservation.Peer.ID)
	}

	if isPeerObservation && peerObservation.Removed {
		r.node.logger.Debug("A node got removed from the cluster", "nodeId", peerObservation.Peer.ID)
	}

	if isResumedHeartbeatObservation {
		r.node.logger.Debug(fmt.Sprintf("A node resumed execution. Peer ID %s ", string(resumedHeartbeatObservation.PeerID)))
		r.node.peerComm.StartJobsOnWorkerNodes()
	}
}

// RemoveNode removes a node from the Raft cluster. Only the leader can perform this operation.
func (r *raftClusterManager) RemoveNode(ctx context.Context, nodeId uint64) error {
	raftObj := r.node.scheduler0RaftStore.GetRaft()
	if raftObj == nil {
		return fmt.Errorf("raft not initialized")
	}

	if raftObj.State() != raft.Leader {
		return fmt.Errorf("node is not leader; cannot remove node")
	}

	serverID := raft.ServerID(strconv.FormatUint(nodeId, 10))
	future := raftObj.RemoveServer(serverID, 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to remove node from raft: %w", err)
	}

	// Unregister from etcd if etcdService is available
	if r.node.etcdService != nil {
		if err := r.node.etcdService.UnregisterNodeById(nodeId); err != nil {
			r.node.logger.Warn("failed to unregister node from etcd", "nodeId", nodeId, "error", err)
			// Don't fail the operation if etcd unregistration fails
		} else {
			r.node.logger.Info("unregistered node from etcd", "nodeId", nodeId)
		}
	}

	r.node.logger.Info("removed node from raft cluster", "nodeId", nodeId)
	return nil
}

// AddNode adds a node to the Raft cluster. Only the leader can perform this operation.
func (r *raftClusterManager) AddNode(ctx context.Context, nodeId uint64, nodeAddress string, clientAddress string) error {
	raftObj := r.node.scheduler0RaftStore.GetRaft()
	if raftObj == nil {
		return fmt.Errorf("raft not initialized")
	}

	if raftObj.State() != raft.Leader {
		return fmt.Errorf("node is not leader; cannot add node")
	}

	serverID := raft.ServerID(strconv.FormatUint(nodeId, 10))
	serverAddr := raft.ServerAddress(nodeAddress)

	// Check if already present
	currentCfg := raftObj.GetConfiguration()
	for _, srv := range currentCfg.Configuration().Servers {
		if srv.ID == serverID {
			// Already in membership; nothing to do
			r.node.logger.Info("node already part of raft cluster", "nodeId", nodeId, "address", nodeAddress)
			return nil
		}
	}

	future := raftObj.AddNonvoter(serverID, serverAddr, 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to add node to raft: %w", err)
	}

	// Register in etcd if etcdService is available
	if r.node.etcdService != nil {
		nodeInfo := config.RaftNode{
			NodeId:        nodeId,
			NodeAddress:   nodeAddress,
			ClientAddress: clientAddress,
		}
		if err := r.node.etcdService.RegisterNode(nodeId, nodeInfo); err != nil {
			r.node.logger.Warn("failed to register node in etcd", "nodeId", nodeId, "error", err)
			// Don't fail the operation if etcd registration fails
		}
	}

	r.node.logger.Info("added node to raft cluster", "nodeId", nodeId, "address", nodeAddress)
	return nil
}

// PromoteNode promotes a non-voter node to a voter in the Raft cluster. Only the leader can perform this operation.
func (r *raftClusterManager) PromoteNode(ctx context.Context, nodeId uint64) error {
	raftObj := r.node.scheduler0RaftStore.GetRaft()
	if raftObj == nil {
		return fmt.Errorf("raft not initialized")
	}

	if raftObj.State() != raft.Leader {
		return fmt.Errorf("node is not leader; cannot promote node")
	}

	serverID := raft.ServerID(strconv.FormatUint(nodeId, 10))

	// Get current configuration to find the node's address
	currentCfg := raftObj.GetConfiguration()
	cfg := currentCfg.Configuration()

	var serverAddress raft.ServerAddress
	var found bool
	var isVoter bool

	for _, srv := range cfg.Servers {
		if srv.ID == serverID {
			serverAddress = srv.Address
			found = true
			isVoter = srv.Suffrage == raft.Voter
			break
		}
	}

	if !found {
		return fmt.Errorf("node %d not found in cluster", nodeId)
	}

	if isVoter {
		r.node.logger.Info("node is already a voter", "nodeId", nodeId)
		return nil
	}

	// Promote the non-voter to voter using AddVoter
	// HashiCorp Raft will handle the promotion automatically
	future := raftObj.AddVoter(serverID, serverAddress, 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to promote node to voter: %w", err)
	}

	r.node.logger.Info("promoted node to voter", "nodeId", nodeId, "address", serverAddress)
	return nil
}

// DemoteNode demotes a voter node to a non-voter in the Raft cluster. Only the leader can perform this operation.
func (r *raftClusterManager) DemoteNode(ctx context.Context, nodeId uint64) error {
	raftObj := r.node.scheduler0RaftStore.GetRaft()
	if raftObj == nil {
		return fmt.Errorf("raft not initialized")
	}

	if raftObj.State() != raft.Leader {
		return fmt.Errorf("node is not leader; cannot demote node")
	}

	serverID := raft.ServerID(strconv.FormatUint(nodeId, 10))

	// Get current configuration to find the node's address
	currentCfg := raftObj.GetConfiguration()
	cfg := currentCfg.Configuration()

	var serverAddress raft.ServerAddress
	var found bool
	var isVoter bool

	for _, srv := range cfg.Servers {
		if srv.ID == serverID {
			serverAddress = srv.Address
			found = true
			isVoter = srv.Suffrage == raft.Voter
			break
		}
	}

	if !found {
		return fmt.Errorf("node %d not found in cluster", nodeId)
	}

	if !isVoter {
		r.node.logger.Info("node is already a non-voter", "nodeId", nodeId)
		return nil
	}

	// Demote the voter to non-voter using AddNonvoter
	// HashiCorp Raft will handle the demotion automatically
	future := raftObj.AddNonvoter(serverID, serverAddress, 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to demote node to non-voter: %w", err)
	}

	r.node.logger.Info("demoted node to non-voter", "nodeId", nodeId, "address", serverAddress)
	return nil
}

// TransferLeadership transfers leadership to another node. Only the leader can perform this operation.
func (r *raftClusterManager) TransferLeadership(ctx context.Context, targetNodeId *uint64) error {
	raftObj := r.node.scheduler0RaftStore.GetRaft()
	if raftObj == nil {
		return fmt.Errorf("raft not initialized")
	}

	if raftObj.State() != raft.Leader {
		return fmt.Errorf("node is not leader; cannot transfer leadership")
	}

	if targetNodeId != nil {
		targetID := raft.ServerID(fmt.Sprintf("%d", *targetNodeId))
		targetAddress := raft.ServerAddress("")

		currentCfg := raftObj.GetConfiguration()
		cfg := currentCfg.Configuration()
		for _, server := range cfg.Servers {
			if server.ID == targetID {
				targetAddress = server.Address
				break
			}
		}

		if targetAddress == "" {
			return fmt.Errorf("target node %d not found in cluster", *targetNodeId)
		}

		future := raftObj.LeadershipTransferToServer(targetID, targetAddress)
		if err := future.Error(); err != nil {
			return fmt.Errorf("failed to transfer leadership to node %d: %w", *targetNodeId, err)
		}
		r.node.logger.Info("transferred leadership to node", "targetNodeId", *targetNodeId)
	} else {
		raftObj.LeadershipTransfer()
		r.node.logger.Info("transferred leadership")
	}

	return nil
}

// ListNodes returns a list of all nodes in the Raft cluster.
func (r *raftClusterManager) ListNodes(ctx context.Context) ([]config.RaftNode, error) {
	raftObj := r.node.scheduler0RaftStore.GetRaft()
	if raftObj == nil {
		return nil, fmt.Errorf("raft not initialized")
	}

	// Get current Raft configuration
	currentCfg := raftObj.GetConfiguration()
	cfg := currentCfg.Configuration()

	nodes := make([]config.RaftNode, 0, len(cfg.Servers))
	for _, server := range cfg.Servers {
		nodeId, err := strconv.ParseUint(string(server.ID), 10, 64)
		if err != nil {
			r.node.logger.Warn("failed to parse node ID from raft server", "serverID", server.ID, "error", err)
			continue
		}

		// Try to get additional info from etcd if available
		var nodeInfo config.RaftNode
		if r.node.etcdService != nil {
			peer, err := r.node.etcdService.GetPeer(nodeId)
			if err == nil && peer != nil {
				nodeInfo = *peer
			} else {
				// Fallback to Raft address if etcd info not available
				nodeInfo = config.RaftNode{
					NodeId:        nodeId,
					NodeAddress:   string(server.Address),
					ClientAddress: string(server.Address),
				}
			}
		} else {
			// No etcd, use Raft address
			nodeInfo = config.RaftNode{
				NodeId:        nodeId,
				NodeAddress:   string(server.Address),
				ClientAddress: string(server.Address),
			}
		}

		nodes = append(nodes, nodeInfo)
	}

	return nodes, nil
}
