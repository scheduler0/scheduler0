package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"scheduler0/pkg/config"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/network"
	"time"

	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"
)

func ConnectRaftLogsAndTransport(muxLn network.Listener, scheduler0Config config.Scheduler0Config) (
	*boltdb.BoltStore,
	*boltdb.BoltStore,
	*raft.FileSnapshotStore,
	raft.Transport,
	network.Listener,
) {
	logger := log.New(os.Stderr, "[get-raft-logs-and-transport] ", log.LstdFlags)

	configs := scheduler0Config.GetConfigurations()
	dirPath := fmt.Sprintf("%v/%v", constants.RaftDir, configs.NodeId)

	logger.Println("creating log store")
	ldb, err := boltdb.NewBoltStore(filepath.Join(dirPath, constants.RaftLog))
	if err != nil {
		logger.Fatal("failed to create log store\n", err)
	}
	logger.Println("creating stable store")
	sdb, err := boltdb.NewBoltStore(filepath.Join(dirPath, constants.RaftStableLog))
	if err != nil {
		logger.Fatal("failed to create stable store\n", err)
	}
	logger.Println("creating snapshot store")
	fss, err := raft.NewFileSnapshotStore(dirPath, 3, os.Stderr)
	if err != nil {
		logger.Fatal("failed to create snapshot store\n", err)
	}

	var tm raft.Transport
	if muxLn != nil {
		logger.Println("creating network transport")
		tm = raft.NewNetworkTransport(network.NewTransport(muxLn), int(configs.RaftTransportMaxPool), time.Second*time.Duration(configs.RaftTransportTimeout), nil)
	}

	return ldb, sdb, fss, tm, muxLn
}
