package cluster

import (
	"database/sql"
	_ "embed"
	"fmt"
	raft "github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/spf13/afero"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/fsm"
	tcp2 "scheduler0/tcp"
	"scheduler0/utils"
	"strconv"
	"time"
)

type clusterData struct {
	Tm  *raft.NetworkTransport
	Ldb *boltdb.BoltStore
	Sdb *boltdb.BoltStore
	Fss *raft.FileSnapshotStore
}

func NewCluster() *clusterData {
	tm, ldb, sdb, fss, err := GetLogsAndTransport()
	if err != nil {
		log.Fatal("failed to create new cluster", err)
	}
	return &clusterData{
		Tm:  tm,
		Ldb: ldb,
		Sdb: sdb,
		Fss: fss,
	}
}

func GetLogsAndTransport() (tm *raft.NetworkTransport, ldb *boltdb.BoltStore, sdb *boltdb.BoltStore, fss *raft.FileSnapshotStore, err error) {
	configs := utils.GetScheduler0Configurations()

	dirPath := fmt.Sprintf("%v/%v", constants.RaftDir, configs.NodeId)
	_, err = strconv.Atoi(configs.RaftSnapshotInterval)
	if err != nil {
		log.Fatal("Failed to convert raft snapshot interval to int", err)
	}

	_, err = strconv.Atoi(configs.RaftSnapshotThreshold)
	if err != nil {
		log.Fatal("Failed to convert raft snapshot threshold to int", err)
	}

	ldb, err = boltdb.NewBoltStore(filepath.Join(dirPath, constants.RaftLog))

	sdb, err = boltdb.NewBoltStore(filepath.Join(dirPath, constants.RaftStableLog))

	fss, err = raft.NewFileSnapshotStore(dirPath, 3, os.Stderr)

	ln, err := net.Listen("tcp", configs.RaftAddress)

	if err != nil {
		utils.CheckErr(err)
	}

	mux := tcp2.NewMux(ln)

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

	tm = raft.NewNetworkTransport(tcp2.NewTransport(muxLn), maxPool, time.Second*time.Duration(timeout), nil)
	return
}

func GetRaftConfigurationFromConfig() raft.Configuration {
	configs := utils.GetScheduler0Configurations()

	servers := []raft.Server{}

	if len(configs.Replicas) > 0 {
		for i, replica := range configs.Replicas {
			servers = append(servers, raft.Server{
				ID:       raft.ServerID(fmt.Sprintf("%v", i+1)),
				Suffrage: raft.Voter,
				Address:  raft.ServerAddress(replica.RaftAddress),
			})
		}
	}
	cfg := raft.Configuration{
		Servers: servers,
	}

	return cfg
}

func NewRaft(fsm raft.FSM, cls *clusterData) (*raft.Raft, error) {
	configs := utils.GetScheduler0Configurations()

	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(configs.NodeId)
	c.ElectionTimeout = time.Duration(rand.Intn(300-150)+300) * time.Millisecond
	c.HeartbeatTimeout = time.Duration(rand.Intn(200-100)+200) * time.Millisecond
	c.LeaderLeaseTimeout = time.Duration(rand.Intn(100-50)+100) * time.Millisecond

	r, err := raft.NewRaft(c, fsm, cls.Ldb, cls.Sdb, cls.Fss, cls.Tm)
	if err != nil {
		return nil, fmt.Errorf("raft.NewRaft: %v", err)
	}
	return r, nil
}

func BootstrapNode(r *raft.Raft) error {
	cfg := GetRaftConfigurationFromConfig()
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

func RecoverRaftStore(cls *clusterData) {
	// Attempt to restore any snapshots we find, newest to oldest.
	log.Println("Recovering node")

	var (
		snapshotIndex  uint64
		snapshotTerm   uint64
		snapshots, err = cls.Fss.List()
	)
	if err != nil {
		utils.CheckErr(err)
	}

	for _, snapshot := range snapshots {
		var source io.ReadCloser
		_, source, err = cls.Fss.Open(snapshot.ID)
		if err != nil {
			// Skip this one and try the next. We will detect if we
			// couldn't open any snapshots.
			continue
		}

		_, err = utils.BytesFromSnapshot(source)
		// Close the source after the restore has completed
		source.Close()
		if err != nil {
			// Same here, skip and try the next one.
			continue
		}

		snapshotIndex = snapshot.Index
		snapshotTerm = snapshot.Term
		break
	}
	if len(snapshots) > 0 && (snapshotIndex == 0 || snapshotTerm == 0) {
		log.Println("failed to restore any of the available snapshots")
		return
	}

	dir, err := os.Getwd()
	if err != nil {
		log.Println(fmt.Errorf("Fatal error getting working dir: %s \n", err))
		return
	}

	recoverDbPath := fmt.Sprintf("%s/%s", dir, "recover.db")

	fs := afero.NewOsFs()

	_, err = fs.Create(recoverDbPath)
	if err != nil {
		log.Println(fmt.Errorf("Fatal db file creation error: %s \n", err))
		return
	}

	dataStore := db.NewSqliteDbConnection(recoverDbPath)
	conn, err := dataStore.OpenConnection()
	dbConnection := conn.(*sql.DB)
	defer dbConnection.Close()

	migrations := db.GetSetupSQL()
	_, err = dbConnection.Exec(migrations)
	if err != nil {
		log.Println(fmt.Errorf("Fatal db file migrations error: %s \n", err))
		return
	}

	fsmStr := fsm.NewFSMStore(dataStore, dbConnection)

	// The snapshot information is the best known end point for the data
	// until we play back the Raft log entries.
	lastIndex := snapshotIndex
	lastTerm := snapshotTerm

	// Apply any Raft log entries past the snapshot.
	lastLogIndex, err := cls.Ldb.LastIndex()
	if err != nil {
		log.Fatalf("failed to find last log: %v", err)
	}

	for index := snapshotIndex + 1; index <= lastLogIndex; index++ {
		var entry raft.Log
		if err = cls.Ldb.GetLog(index, &entry); err != nil {
			log.Fatalf("failed to get log at index %d: %v\n", index, err)
		}
		if entry.Type == raft.LogCommand {
			fsm.ApplyCommand(&entry, dbConnection)
		}
		lastIndex = entry.Index
		lastTerm = entry.Term
	}

	rows, err := dbConnection.Query("select count() from jobs")
	defer rows.Close()
	if err != nil {
		log.Fatalf("failed to read recovery:db: %v", err)
	}
	var count int
	for rows.Next() {
		err := rows.Scan(&count)
		if err != nil {
			log.Fatalf("failed to read recovery:db: %v", err)
		}
	}
	if rows.Err() != nil {
		if err != nil {
			log.Fatalf("failed to read recovery:db: %v", err)
		}
	}

	log.Println("Wrote number of jobs to recovery db ::", count)

	cfg := GetRaftConfigurationFromConfig()

	snapshot := fsm.NewFSMSnapshot(fsmStr.SqliteDB)
	sink, err := cls.Fss.Create(1, lastIndex, lastTerm, cfg, 1, cls.Tm)
	if err != nil {
		log.Fatalf("failed to create snapshot: %v", err)
	}
	if err = snapshot.Persist(sink); err != nil {
		log.Fatalf("failed to persist snapshot: %v", err)
	}
	if err = sink.Close(); err != nil {
		log.Fatalf("failed to finalize snapshot: %v", err)
	}

	firstLogIndex, err := cls.Ldb.FirstIndex()
	if err != nil {
		log.Fatalf("failed to get first log index: %v", err)
	}
	if err := cls.Ldb.DeleteRange(firstLogIndex, lastLogIndex); err != nil {
		log.Fatalf("log compaction failed: %v", err)
	}

	err = os.Remove(recoverDbPath)
	if err != nil {
		log.Fatalf("failed to delete recovery db: %v", err)
	}
}
