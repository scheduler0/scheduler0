package peer

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

type peer struct {
	Tm     *raft.NetworkTransport
	Ldb    *boltdb.BoltStore
	Sdb    *boltdb.BoltStore
	Fss    *raft.FileSnapshotStore
	logger *log.Logger
}

func NewPeer(logger *log.Logger) *peer {
	logPrefix := logger.Prefix()
	logger.SetPrefix(fmt.Sprintf("%s[creating-new-peer] ", logPrefix))
	defer logger.SetPrefix(logPrefix)

	tm, ldb, sdb, fss, err := getLogsAndTransport(logger)
	if err != nil {
		logger.Fatal("failed essentials for peer", err)
	}
	return &peer{
		Tm:     tm,
		Ldb:    ldb,
		Sdb:    sdb,
		Fss:    fss,
		logger: logger,
	}
}

func (p *peer) NewRaft(fsm raft.FSM) *raft.Raft {
	logPrefix := p.logger.Prefix()
	p.logger.SetPrefix(fmt.Sprintf("%s[creating-new-peer-raft] ", logPrefix))
	defer p.logger.SetPrefix(logPrefix)

	configs := utils.GetScheduler0Configurations()

	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(configs.NodeId)
	c.ElectionTimeout = time.Duration(rand.Intn(300-150)+300) * time.Millisecond
	c.HeartbeatTimeout = time.Duration(rand.Intn(200-100)+200) * time.Millisecond
	c.LeaderLeaseTimeout = time.Duration(rand.Intn(100-50)+100) * time.Millisecond

	r, err := raft.NewRaft(c, fsm, p.Ldb, p.Sdb, p.Fss, p.Tm)
	if err != nil {
		p.logger.Fatalln("failed to create raft object for peer", err)
	}
	return r
}

func (p *peer) BootstrapNode(r *raft.Raft) {
	logPrefix := p.logger.Prefix()
	p.logger.SetPrefix(fmt.Sprintf("%s[boostraping-raft-cluster] ", logPrefix))
	defer p.logger.SetPrefix(logPrefix)

	cfg := getRaftConfigurationFromConfig()
	f := r.BootstrapCluster(cfg)
	if err := f.Error(); err != nil {
		p.logger.Fatalln("failed to bootstrap raft peer", err)
	}
}

func (p *peer) RecoverPeer() {
	logPrefix := p.logger.Prefix()
	p.logger.SetPrefix(fmt.Sprintf("%s[recovering-peer] ", logPrefix))
	defer p.logger.SetPrefix(logPrefix)

	var (
		snapshotIndex  uint64
		snapshotTerm   uint64
		snapshots, err = p.Fss.List()
	)
	if err != nil {
		utils.CheckErr(err)
	}

	for _, snapshot := range snapshots {
		var source io.ReadCloser
		_, source, err = p.Fss.Open(snapshot.ID)
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
		p.logger.Println("failed to restore any of the available snapshots")
		return
	}

	dir, err := os.Getwd()
	if err != nil {
		p.logger.Println(fmt.Errorf("Fatal error getting working dir: %s \n", err))
		return
	}

	recoverDbPath := fmt.Sprintf("%s/%s", dir, "recover.db")

	fs := afero.NewOsFs()

	_, err = fs.Create(recoverDbPath)
	if err != nil {
		p.logger.Println(fmt.Errorf("Fatal db file creation error: %s \n", err))
		return
	}

	dataStore := db.NewSqliteDbConnection(recoverDbPath)
	conn, err := dataStore.OpenConnection()
	dbConnection := conn.(*sql.DB)
	defer dbConnection.Close()

	migrations := db.GetSetupSQL()
	_, err = dbConnection.Exec(migrations)
	if err != nil {
		p.logger.Println(fmt.Errorf("Fatal db file migrations error: %s \n", err))
		return
	}

	fsmStr := fsm.NewFSMStore(dataStore, dbConnection, p.logger)

	// The snapshot information is the best known end point for the data
	// until we play back the Raft log entries.
	lastIndex := snapshotIndex
	lastTerm := snapshotTerm

	// Apply any Raft log entries past the snapshot.
	lastLogIndex, err := p.Ldb.LastIndex()
	if err != nil {
		p.logger.Fatalf("failed to find last log: %v", err)
	}

	for index := snapshotIndex + 1; index <= lastLogIndex; index++ {
		var entry raft.Log
		if err = p.Ldb.GetLog(index, &entry); err != nil {
			p.logger.Fatalf("failed to get log at index %d: %v\n", index, err)
		}
		if entry.Type == raft.LogCommand {
			fsm.ApplyCommand(&entry, dbConnection, false, nil)
		}
		lastIndex = entry.Index
		lastTerm = entry.Term
	}

	rows, err := dbConnection.Query("select count() from jobs")
	defer rows.Close()
	if err != nil {
		p.logger.Fatalf("failed to read recovery:db: %v", err)
	}
	var count int
	for rows.Next() {
		err := rows.Scan(&count)
		if err != nil {
			p.logger.Fatalf("failed to read recovery:db: %v", err)
		}
	}
	if rows.Err() != nil {
		if err != nil {
			p.logger.Fatalf("failed to read recovery:db: %v", err)
		}
	}

	p.logger.Println("Wrote number of jobs to recovery db ::", count)

	cfg := getRaftConfigurationFromConfig()

	snapshot := fsm.NewFSMSnapshot(fsmStr.SqliteDB)
	sink, err := p.Fss.Create(1, lastIndex, lastTerm, cfg, 1, p.Tm)
	if err != nil {
		p.logger.Fatalf("failed to create snapshot: %v", err)
	}
	if err = snapshot.Persist(sink); err != nil {
		p.logger.Fatalf("failed to persist snapshot: %v", err)
	}
	if err = sink.Close(); err != nil {
		p.logger.Fatalf("failed to finalize snapshot: %v", err)
	}

	firstLogIndex, err := p.Ldb.FirstIndex()
	if err != nil {
		p.logger.Fatalf("failed to get first log index: %v", err)
	}
	if err := p.Ldb.DeleteRange(firstLogIndex, lastLogIndex); err != nil {
		p.logger.Fatalf("log compaction failed: %v", err)
	}

	err = os.Remove(recoverDbPath)
	if err != nil {
		p.logger.Fatalf("failed to delete recovery db: %v", err)
	}
}

func getLogsAndTransport(logger *log.Logger) (tm *raft.NetworkTransport, ldb *boltdb.BoltStore, sdb *boltdb.BoltStore, fss *raft.FileSnapshotStore, err error) {
	logPrefix := logger.Prefix()
	logger.SetPrefix(fmt.Sprintf("%s[creating-new-peer-essentials] ", logPrefix))
	defer logger.SetPrefix(logPrefix)

	configs := utils.GetScheduler0Configurations()

	dirPath := fmt.Sprintf("%v/%v", constants.RaftDir, configs.NodeId)
	_, err = strconv.Atoi(configs.RaftSnapshotInterval)
	if err != nil {
		logger.Fatal("Failed to convert raft snapshot interval to int", err)
	}

	_, err = strconv.Atoi(configs.RaftSnapshotThreshold)
	if err != nil {
		logger.Fatal("Failed to convert raft snapshot threshold to int", err)
	}

	ldb, err = boltdb.NewBoltStore(filepath.Join(dirPath, constants.RaftLog))

	sdb, err = boltdb.NewBoltStore(filepath.Join(dirPath, constants.RaftStableLog))

	fss, err = raft.NewFileSnapshotStore(dirPath, 3, os.Stderr)

	ln, err := net.Listen("tcp", configs.RaftAddress)
	if err != nil {
		logger.Fatal("failed to listen to tcp net", err)
	}

	mux := tcp2.NewMux(ln)

	go mux.Serve()

	muxLn := mux.Listen(1)

	maxPool, err := strconv.Atoi(configs.RaftTransportMaxPool)
	if err != nil {
		logger.Fatal("Failed to convert raft transport max pool to int", err)
	}

	timeout, err := strconv.Atoi(configs.RaftTransportTimeout)
	if err != nil {
		logger.Fatal("Failed to convert raft transport timeout to int", err)
	}

	tm = raft.NewNetworkTransport(tcp2.NewTransport(muxLn), maxPool, time.Second*time.Duration(timeout), nil)
	return
}

func getRaftConfigurationFromConfig() raft.Configuration /**/ {
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
