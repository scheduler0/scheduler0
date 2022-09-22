package cluster

import (
	"fmt"
	raft "github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"
	"log"
	"net"
	"os"
	"path/filepath"
	"scheduler0/constants"
	"scheduler0/server/fsm"
	"scheduler0/server/tcp"
	"scheduler0/utils"
	"strconv"
	"time"
)

func NewRaft(raftDir string, fsm raft.FSM) (*raft.Raft, *raft.NetworkTransport, *boltdb.BoltStore, *boltdb.BoltStore, *raft.FileSnapshotStore, error) {
	configs := utils.GetScheduler0Configurations()

	_, err := strconv.Atoi(configs.RaftSnapshotInterval)
	if err != nil {
		log.Fatal("Failed to convert raft snapshot interval to int", err)
	}

	_, err = strconv.Atoi(configs.RaftSnapshotThreshold)
	if err != nil {
		log.Fatal("Failed to convert raft snapshot threshold to int", err)
	}

	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(configs.NodeId)
	//c.SnapshotThreshold = uint64(threshold)
	//c.SnapshotInterval = time.Duration(120) * time.Second
	//c.ElectionTimeout = time.Duration(rand.Intn(8-5)+8) * time.Second
	//c.HeartbeatTimeout = c.ElectionTimeout
	//c.LeaderLeaseTimeout = time.Second * 5

	baseDir := raftDir

	ldb, err := boltdb.NewBoltStore(filepath.Join(baseDir, constants.RaftLog))
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf(`boltdb.NewBoltStore(%q): %v`, filepath.Join(baseDir, constants.RaftLog), err)
	}

	sdb, err := boltdb.NewBoltStore(filepath.Join(baseDir, constants.RaftStableLog))
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf(`boltdb.NewBoltStore(%q): %v`, filepath.Join(baseDir, constants.RaftStableLog), err)
	}

	fss, err := raft.NewFileSnapshotStore(baseDir, 3, os.Stderr)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf(`raft.NewFileSnapshotStore(%q, ...): %v`, baseDir, err)
	}

	ln, err := net.Listen("tcp", configs.RaftAddress)

	if err != nil {
		utils.CheckErr(err)
	}

	mux := tcp.NewMux(ln)

	go mux.Serve()

	muxLn := mux.Listen(1)

	maxPool, err := strconv.Atoi(configs.RaftTransportMaxPool)
	if err != nil {
		log.Fatal("Failed to convert raft transport max pool to int", err)
	}

	timeout, err := strconv.Atoi(configs.RaftTransportTimeout)
	if err != nil {
		log.Fatal("Failed to convert raft transport timeout to int", err)
	}

	tm := raft.NewNetworkTransport(tcp.NewTransport(muxLn), maxPool, time.Second*time.Duration(timeout), nil)

	r, err := raft.NewRaft(c, fsm, ldb, sdb, fss, tm)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("raft.NewRaft: %v", err)
	}

	return r, tm, ldb, sdb, fss, nil
}

func BootstrapNode(r *raft.Raft) error {
	configs := utils.GetScheduler0Configurations()
	servers := []raft.Server{
		raft.Server{
			ID:       raft.ServerID(fmt.Sprintf("%v", configs.NodeId)),
			Suffrage: raft.Voter,
			Address:  raft.ServerAddress(configs.RaftAddress),
		},
	}

	//if len(configs.Replicas) > 0 {
	//	for i, replica := range configs.Replicas {
	//		servers = append(servers, raft.Server{
	//			ID:       raft.ServerID(fmt.Sprintf("%v", i+3)),
	//			Suffrage: raft.Voter,
	//			Address:  raft.ServerAddress(replica),
	//		})
	//	}
	//}

	cfg := raft.Configuration{
		Servers: servers,
	}

	f := r.BootstrapCluster(cfg)
	if err := f.Error(); err != nil {
		return fmt.Errorf("raft.Raft.BootstrapCluster: %v", err)
	}

	return nil
}

func RecoverCluster(fsm raft.FSM, tm *raft.NetworkTransport, configuration raft.Configuration) {
	configs := utils.GetScheduler0Configurations()

	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(configs.NodeId)

	baseDir := "raft_data"

	ldb, err := boltdb.NewBoltStore(filepath.Join(baseDir, constants.RaftLog))
	if err != nil {
		utils.Error(`boltdb.NewBoltStore(%q): %v`, filepath.Join(baseDir, constants.RaftLog), err)
	}

	sdb, err := boltdb.NewBoltStore(filepath.Join(baseDir, constants.RaftStableLog))
	if err != nil {
		utils.Error(`boltdb.NewBoltStore(%q): %v`, filepath.Join(baseDir, constants.RaftStableLog), err)
	}

	fss, err := raft.NewFileSnapshotStore(baseDir, 3, os.Stderr)
	if err != nil {
		utils.Error(`raft.NewFileSnapshotStore(%q, ...): %v`, baseDir, err)
	}

	err = raft.RecoverCluster(c, fsm, ldb, sdb, fss, tm, configuration)
	if err != nil {
		utils.Error("Failed to recover cluster", err.Error())
	}
}

func RecoverRaftStore(fsmStr *fsm.Store, logs *boltdb.BoltStore, tn *raft.NetworkTransport, snaps *raft.FileSnapshotStore) {
	// Attempt to restore any snapshots we find, newest to oldest.

	log.Println("Recovering node")

	lastIndex, err := logs.LastIndex()
	if err != nil {
		return
	}

	fmt.Println("lastIndex", lastIndex)

	//var (
	//	snapshotIndex  uint64
	//	snapshotTerm   uint64
	//	snapshots, err = snaps.List()
	//)
	//if err != nil {
	//	utils.CheckErr(err)
	//}
	//
	//for _, snapshot := range snapshots {
	//	var source io.ReadCloser
	//	_, source, err = snaps.Open(snapshot.ID)
	//	if err != nil {
	//		// Skip this one and try the next. We will detect if we
	//		// couldn't open any snapshots.
	//		continue
	//	}
	//
	//	_, err = utils.BytesFromSnapshot(source)
	//	// Close the source after the restore has completed
	//	source.Close()
	//	if err != nil {
	//		// Same here, skip and try the next one.
	//		continue
	//	}
	//
	//	snapshotIndex = snapshot.Index
	//	snapshotTerm = snapshot.Term
	//	break
	//}
	//if len(snapshots) > 0 && (snapshotIndex == 0 || snapshotTerm == 0) {
	//	log.Fatalln("failed to restore any of the available snapshots")
	//}
	//
	//inMemDb, err := db.NewMemSqliteDd()
	//defer inMemDb.Close()
	//
	//// The snapshot information is the best known end point for the data
	//// until we play back the Raft log entries.
	//lastIndex := snapshotIndex
	//lastTerm := snapshotTerm
	//
	//// Apply any Raft log entries past the snapshot.
	//lastLogIndex, err := logs.LastIndex()
	//if err != nil {
	//	log.Fatalf("failed to find last log: %v", err)
	//}
	//
	//for index := snapshotIndex + 1; index <= lastLogIndex; index++ {
	//	var entry raft.Log
	//	if err = logs.GetLog(index, &entry); err != nil {
	//		log.Fatalf("failed to get log at index %d: %v\n", index, err)
	//	}
	//	if entry.Type == raft.LogCommand {
	//		dbConnection := inMemDb.(*sql.DB)
	//		fsmStore := fsm.NewFSMStore(fsmStr.SqliteDB, dbConnection)
	//		fsmStore.Apply(&entry)
	//	}
	//	lastIndex = entry.Index
	//	lastTerm = entry.Term
	//}
	//
	////configs := utils.GetScheduler0Configurations()
	//
	//conf := fsmStr.Raft.GetConfiguration().Configuration()
	//
	//fmt.Println("conf", conf)
	//
	//snapshot := fsm.NewFSMSnapshot(fsmStr.SqliteDB)
	//sink, err := snaps.Create(1, lastIndex, lastTerm, conf, 1, tn)
	//if err != nil {
	//	log.Fatalf("failed to create snapshot: %v", err)
	//}
	//if err = snapshot.Persist(sink); err != nil {
	//	log.Fatalf("failed to persist snapshot: %v", err)
	//}
	//if err = sink.Close(); err != nil {
	//	log.Fatalf("failed to finalize snapshot: %v", err)
	//}
	//
	//firstLogIndex, err := logs.FirstIndex()
	//if err != nil {
	//	log.Fatalf("failed to get first log index: %v", err)
	//}
	//if err := logs.DeleteRange(firstLogIndex, lastLogIndex); err != nil {
	//	log.Fatalf("log compaction failed: %v", err)
	//}
}
