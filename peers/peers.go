package peers

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb/v2"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/db"
	"scheduler0/fsm"
	"scheduler0/headers"
	"scheduler0/job_executor"
	"scheduler0/job_process"
	"scheduler0/job_queue"
	"scheduler0/models"
	"scheduler0/repository"
	"scheduler0/secrets"
	tcp2 "scheduler0/tcp"
	"scheduler0/utils"
	"strconv"
	"sync"
	"time"
)

type PeerStatus struct {
	IsLeader           bool
	IsAuth             bool
	IsAlive            bool
	LastConnectionTime time.Duration
}

type PeerRequest struct{}

type PeerRes struct {
	IsLeader bool
}

type PeerResponse struct {
	Data    PeerRes `json:"data"`
	Success bool    `json:"success"`
}

type PeerState int

const (
	Cold          PeerState = 0
	Bootstrapping           = 1
	Ready                   = 2
	ShuttingDown            = 3
	Unstable                = 4
)

type Peer struct {
	Tm            *raft.NetworkTransport
	Ldb           *boltdb.BoltStore
	Sdb           *boltdb.BoltStore
	Fss           *raft.FileSnapshotStore
	logger        *log.Logger
	Neighbors     map[string]PeerStatus
	mtx           sync.Mutex
	AcceptWrites  bool
	State         PeerState
	queue         chan []PeerRequest
	Rft           *raft.Raft
	jobProcessor  *job_process.JobProcessor
	jobQueue      *job_queue.JobQueue
	jobExecutor   *job_executor.JobExecutor
	jobRepo       repository.Job
	projectRepo   repository.Project
	ExistingNode  bool
	LeaderAddress string
}

func NewPeer(
	logger *log.Logger,
	jobExecutor *job_executor.JobExecutor,
	jobQueue *job_queue.JobQueue,
	jobRepo repository.Job,
	projectRepo repository.Project,
) *Peer {
	logPrefix := logger.Prefix()
	logger.SetPrefix(fmt.Sprintf("%s[creating-new-Peer] ", logPrefix))
	defer logger.SetPrefix(logPrefix)
	dirPath := fmt.Sprintf("%v", constants.RaftDir)
	dirPath, exists := utils.MakeDirIfNotExist(logger, dirPath)
	configs := config.GetScheduler0Configurations(logger)
	dirPath = fmt.Sprintf("%v/%v", constants.RaftDir, configs.NodeId)
	utils.MakeDirIfNotExist(logger, dirPath)
	tm, ldb, sdb, fss, err := getLogsAndTransport(logger)
	if err != nil {
		logger.Fatal("failed essentials for Peer", err)
	}

	return &Peer{
		Tm:           tm,
		Ldb:          ldb,
		Sdb:          sdb,
		Fss:          fss,
		logger:       logger,
		Neighbors:    map[string]PeerStatus{},
		AcceptWrites: false,
		State:        Cold,
		jobProcessor: job_process.NewJobProcessor(jobRepo, projectRepo, *jobQueue, logger),
		jobQueue:     jobQueue,
		jobExecutor:  jobExecutor,
		jobRepo:      jobRepo,
		projectRepo:  projectRepo,
		ExistingNode: exists,
	}
}

func (p *Peer) NewRaft(fsm raft.FSM) *raft.Raft {
	logPrefix := p.logger.Prefix()
	p.logger.SetPrefix(fmt.Sprintf("%s[creating-new-Peer-raft] ", logPrefix))
	defer p.logger.SetPrefix(logPrefix)
	configs := config.GetScheduler0Configurations(p.logger)

	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(configs.NodeId)

	// TODO: Set raft configs in scheduler0 config

	r, err := raft.NewRaft(c, fsm, p.Ldb, p.Sdb, p.Fss, p.Tm)
	if err != nil {
		p.logger.Fatalln("failed to create raft object for Peer", err)
	}
	return r
}

func (p *Peer) BootstrapRaftCluster(r *raft.Raft) {
	logPrefix := p.logger.Prefix()
	p.logger.SetPrefix(fmt.Sprintf("%s[boostraping-raft-cluster] ", logPrefix))
	defer p.logger.SetPrefix(logPrefix)

	cfg := p.getRaftConfigurationFromConfig(p.logger)
	f := r.BootstrapCluster(cfg)
	if err := f.Error(); err != nil {
		p.logger.Fatalln("failed to bootstrap raft Peer", err)
	}
}

func (p *Peer) RecoverRaftState() {
	logPrefix := p.logger.Prefix()
	p.logger.SetPrefix(fmt.Sprintf("%s[recovering-Peer] ", logPrefix))
	defer p.logger.SetPrefix(logPrefix)

	var (
		snapshotIndex  uint64
		snapshotTerm   uint64
		snapshots, err = p.Fss.List()
	)
	if err != nil {
		p.logger.Fatalln(err)
	}

	p.logger.Println("Found ", len(snapshots), " snapshots")

	var lastSnapshotBytes []byte
	for _, snapshot := range snapshots {
		var source io.ReadCloser
		_, source, err = p.Fss.Open(snapshot.ID)
		if err != nil {
			// Skip this one and try the next. We will detect if we
			// couldn't open any snapshots.
			continue
		}

		lastSnapshotBytes, err = utils.BytesFromSnapshot(source)
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

	err = os.WriteFile(recoverDbPath, lastSnapshotBytes, os.ModePerm)
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
	var lastConfiguration raft.Configuration
	var lastConfigurationIndex uint64
	for index := snapshotIndex + 1; index <= lastLogIndex; index++ {
		var entry raft.Log
		if err = p.Ldb.GetLog(index, &entry); err != nil {
			p.logger.Fatalf("failed to get log at index %d: %v\n", index, err)
		}
		fsm.ApplyCommand(p.logger, &entry, dbConnection, false, nil, nil, nil, nil)
		if entry.Type == raft.LogConfiguration {
			lastConfigurationIndex = index
			lastConfiguration = raft.DecodeConfiguration(entry.Data)
		}
		lastIndex = entry.Index
		lastTerm = entry.Term
	}

	//---count jobs

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

	//---count credentials

	rows, err = dbConnection.Query("select count() from credentials")
	defer rows.Close()
	if err != nil {
		p.logger.Fatalf("failed to read recovery:db: %v", err)
	}
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

	p.logger.Println("Wrote number of credentials to recovery db ::", count)

	snapshot := fsm.NewFSMSnapshot(fsmStr.SqliteDB)
	sink, err := p.Fss.Create(1, lastIndex, lastTerm, lastConfiguration, lastConfigurationIndex, p.Tm)
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

func (p *Peer) BoostrapPeer(fsmStr *fsm.Store) {
	configs := config.GetScheduler0Configurations(p.logger)
	p.State = Bootstrapping
	if configs.Bootstrap == "true" {
		p.jobExecutor.LeaderAddress = fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port)
		p.AuthenticateWithPeersInConfig(p.logger)
	}
	if p.ExistingNode {
		p.logger.Println("discovered existing raft dir")
		p.RecoverRaftState()
	}
	rft := p.NewRaft(fsmStr)
	if configs.Bootstrap == "true" && !p.ExistingNode {
		p.BootstrapRaftCluster(rft)
	}
	fsmStr.Raft = rft
	p.Rft = rft
	go p.ShardCronJobs()
	go p.jobExecutor.ListenToChannelsUpdates()
	p.RunPendingCronJob(fsmStr)
}

func (p *Peer) AddNewPeerNeighbor() {}

func (p *Peer) RemovePeerNeighbor(peerAddress string) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	delete(p.Neighbors, peerAddress)
}

func (p *Peer) AuthenticateWithPeersInConfig(logger *log.Logger) {
	p.logger.Println("Authenticating with peers...")

	configs := config.GetScheduler0Configurations(logger)
	var wg sync.WaitGroup

	results := map[string]PeerStatus{}
	wrlck := sync.Mutex{}

	for _, replica := range configs.Replicas {
		if replica.Address != fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port) {
			wg.Add(1)
			go func(rep config.Peer, res map[string]PeerStatus) {
				wrlck.Lock()
				err := utils.RetryOnError(func() error {
					if peerStatus, err := connectPeer(logger, rep); err == nil {
						results[rep.Address] = *peerStatus
					} else {
						return err
					}

					return nil
				}, configs.PeerConnectRetryMax, configs.PeerConnectRetryDelay)
				wg.Done()
				wrlck.Unlock()
				if err != nil {
					p.logger.Println("failed to authenticate with Peer ", rep.Address)
				}

				logger.Println("failed to connect with Peer error:", err)
			}(replica, results)
		}
	}
	wg.Wait()

	p.mtx.Lock()
	defer p.mtx.Unlock()

	for addr, result := range results {
		_, ok := p.Neighbors[addr]
		if ok {
			p.Neighbors[addr] = PeerStatus{
				IsAlive:            result.IsAlive,
				IsLeader:           result.IsLeader,
				LastConnectionTime: result.LastConnectionTime,
				IsAuth:             result.IsAuth,
			}
		} else {
			p.Neighbors[addr] = result
		}
	}

	p.AcceptWrites = true
}

func (p *Peer) EnsureSingleBootstrapConfig() {}

func (p *Peer) ConsolidateLeaderFromPeers() {}

func (p *Peer) StartFailureDetector() {}

func (p *Peer) BroadcastDetectedFailure() {}

func (p *Peer) ShardCronJobs() {
	p.logger.Println("begin sharing cron jobs")
	configs := config.GetScheduler0Configurations(p.logger)
	ticker := time.NewTicker(time.Duration(configs.PeerCronJobCheckInterval) * time.Millisecond)
	startedJobs := false

	for !startedJobs {
		select {
		case <-p.Rft.LeaderCh():
			leaderAddress, _ := p.Rft.LeaderWithID()
			if p.LeaderAddress != string(leaderAddress) {
				// TODO: Get leader address from configs
				p.LeaderAddress = string(leaderAddress)
				p.jobExecutor.LeaderAddress = string(leaderAddress)
				p.jobExecutor.StopAll()
			}
		case <-ticker.C:
			if p.Rft.State() == raft.Shutdown {
				return
			}
			vErr := p.Rft.VerifyLeader()
			if vErr.Error() == nil {
				p.logger.Println("starting jobs")
				go p.jobProcessor.StartJobs()
				startedJobs = true
			}
		}
	}
}

func (p *Peer) RunPendingCronJob(fsmStr *fsm.Store) {
	p.logger.Println("begin listening for jobs")

	go func() {
		for {
			select {
			case pendingJob := <-fsmStr.PendingJobs:
				p.jobExecutor.Run(pendingJob)
			case preparedJob := <-fsmStr.PrepareJobs:
				p.jobExecutor.LogPrepare(preparedJob)
			case commitJob := <-fsmStr.CommitJobs:
				p.jobExecutor.LogCommit(commitJob)
			case errorJob := <-fsmStr.ErrorJobs:
				p.jobExecutor.LogErrors(errorJob)
			}
		}
	}()
}

func (p *Peer) LogJobsStatePeers(peerAddress string, pendingJobs []models.JobModel, actionType constants.Command) {
	if p.Rft == nil {
		p.logger.Fatalln("raft is not set on job executors")
	}

	data := []interface{}{}

	for _, pendingJob := range pendingJobs {
		data = append(data, pendingJob)
	}

	_, applyErr := fsm.AppApply(
		p.logger,
		p.Rft,
		actionType,
		peerAddress,
		data,
	)
	if applyErr != nil {
		p.logger.Fatalln("failed to apply job update states ", applyErr)
	}
}

func (p *Peer) SetLeaderAddress(leaderAddress string) {
	p.LeaderAddress = leaderAddress
	p.jobExecutor.LeaderAddress = leaderAddress
}

func (p *Peer) getRaftConfigurationFromConfig(logger *log.Logger) raft.Configuration {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	configs := config.GetScheduler0Configurations(logger)
	servers := []raft.Server{}

	servers = append(servers, raft.Server{
		ID:       raft.ServerID(configs.NodeId),
		Suffrage: raft.Voter,
		Address:  raft.ServerAddress(configs.RaftAddress),
	})

	for _, replica := range configs.Replicas {
		if repStatus, ok := p.Neighbors[replica.Address]; ok &&
			repStatus.IsAlive &&
			repStatus.IsAuth {
			servers = append(servers, raft.Server{
				ID:       raft.ServerID(replica.NodeId),
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

func connectPeer(logger *log.Logger, rep config.Peer) (*PeerStatus, error) {
	configs := config.GetScheduler0Configurations(logger)
	httpClient := http.Client{
		Timeout: time.Duration(configs.PeerAuthRequestTimeout) * time.Second,
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/peer-handshake", rep.Address), nil)
	if err != nil {
		logger.Println("failed to create request ", err)
		return nil, err
	}
	req.Header.Set(headers.PeerHeader, "peer")
	req.Header.Set(headers.PeerAddressHeader, fmt.Sprintf("%s://%s:%s", configs.Protocol, configs.Host, configs.Port))
	credentials := secrets.GetSecrets(logger)
	req.SetBasicAuth(credentials.AuthUsername, credentials.AuthPassword)

	start := time.Now()

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Println("failed to send request ", err)
		return nil, err
	}
	defer resp.Body.Close()

	connectionTime := time.Since(start)

	if resp.StatusCode == http.StatusOK {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Println("failed to response", err)
			return nil, err
		}

		body := PeerResponse{}

		err = json.Unmarshal(data, &body)
		if err != nil {
			logger.Println("failed to unmarshal response ", err)
			return nil, err
		}

		logger.Println("successfully authenticated ", rep.Address, " body ", body)

		return &PeerStatus{
			IsAlive:            true,
			IsAuth:             true,
			IsLeader:           body.Data.IsLeader,
			LastConnectionTime: connectionTime,
		}, nil
	}

	logger.Println("could not authenticate ", rep.Address, " status code:", resp.StatusCode)

	if resp.StatusCode == http.StatusUnauthorized {
		return &PeerStatus{
			IsAlive:            true,
			IsAuth:             false,
			IsLeader:           false,
			LastConnectionTime: connectionTime,
		}, nil
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		return &PeerStatus{
			IsAlive:            false,
			IsAuth:             false,
			IsLeader:           false,
			LastConnectionTime: connectionTime,
		}, nil
	}

	return &PeerStatus{
		IsAlive:            false,
		IsAuth:             false,
		IsLeader:           false,
		LastConnectionTime: connectionTime,
	}, nil
}

func getLogsAndTransport(logger *log.Logger) (tm *raft.NetworkTransport, ldb *boltdb.BoltStore, sdb *boltdb.BoltStore, fss *raft.FileSnapshotStore, err error) {
	logPrefix := logger.Prefix()
	logger.SetPrefix(fmt.Sprintf("%s[creating-new-Peer-essentials] ", logPrefix))
	defer logger.SetPrefix(logPrefix)

	configs := config.GetScheduler0Configurations(logger)

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
	if err != nil {
		logger.Fatal("failed to create log store", err)
	}
	sdb, err = boltdb.NewBoltStore(filepath.Join(dirPath, constants.RaftStableLog))
	if err != nil {
		logger.Fatal("failed to create stable store", err)
	}
	fss, err = raft.NewFileSnapshotStore(dirPath, 3, os.Stderr)
	if err != nil {
		logger.Fatal("failed to create snapshot store", err)
	}
	ln, err := net.Listen("tcp", configs.RaftAddress)
	if err != nil {
		logger.Fatal("failed to listen to tcp net", err)
	}

	mux := tcp2.NewMux(logger, ln)

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
