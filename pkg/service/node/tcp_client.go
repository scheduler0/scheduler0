package node

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"scheduler0/pkg/config"
	"scheduler0/pkg/models"
	"scheduler0/pkg/network"
	"scheduler0/pkg/secrets"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"
)

type tcpClient struct {
	logger            hclog.Logger
	ln                network.Listener
	scheduler0Configs config.Scheduler0Config
	scheduler0Secrets secrets.Scheduler0Secrets
}

const (
	tcpClientMaxRetries     = 4
	tcpClientBaseBackoff    = 200 * time.Millisecond
	tcpClientMaxBackoff     = 5 * time.Second
	tcpClientJitterFraction = 0.5 // 50% of the current backoff
)

func (client tcpClient) withRetry(ctx context.Context, opName string, fn func() error) error {
	backoff := tcpClientBaseBackoff
	var lastErr error

	for attempt := 1; attempt <= tcpClientMaxRetries; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			client.logger.Error("tcp client operation canceled by context", "operation", opName, "error", ctx.Err())
			return ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			if attempt > 1 {
				client.logger.Info("tcp client operation succeeded after retries", "operation", opName, "attempt", attempt)
			}
			return nil
		}

		// If non-retriable error or we've exhausted retries, return immediately
		if !isRetriableTCPErr(err) || attempt == tcpClientMaxRetries {
			client.logger.Error("tcp client operation failed", "operation", opName, "attempt", attempt, "error", err)
			return err
		}

		lastErr = err

		// Compute exponential backoff with jitter
		jitterRange := time.Duration(float64(backoff) * tcpClientJitterFraction)
		var jitter time.Duration
		if jitterRange > 0 {
			jitter = time.Duration(rand.Int63n(int64(jitterRange)))
		}
		sleep := backoff + jitter

		client.logger.Info("retrying tcp client operation", "operation", opName, "attempt", attempt, "error", err, "backoff", sleep)

		select {
		case <-ctx.Done():
			client.logger.Error("tcp client operation canceled during backoff", "operation", opName, "error", ctx.Err())
			return ctx.Err()
		case <-time.After(sleep):
		}

		backoff *= 2
		if backoff > tcpClientMaxBackoff {
			backoff = tcpClientMaxBackoff
		}
	}

	return lastErr
}

func isRetriableTCPErr(err error) bool {
	if err == nil {
		return false
	}

	// Do not retry on explicit context cancellation / deadline
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Unwrap net.Error anywhere in the chain
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() || netErr.Temporary() {
			return true
		}
	}

	// Connection reset by peer and similar low-level connection errors
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}

	if strings.Contains(strings.ToLower(err.Error()), "connection reset by peer") {
		return true
	}

	// EOF on network read/write can be transient in distributed systems
	if errors.Is(err, io.EOF) {
		return true
	}

	return false
}

func (client tcpClient) StopJobs(ctx context.Context, node *nodeService, peer config.RaftNode) error {
	return client.withRetry(ctx, "StopJobs", func() error {
		configs := client.scheduler0Configs.GetConfigurations()
		conn, err := client.ln.Dial(peer.NodeAddress, time.Duration(configs.RaftTransportTimeout)*time.Second)
		if err != nil {
			client.logger.Error("failed to open connection to peer node", "error", err, "address", peer.NodeAddress)
			return err
		}
		defer conn.Close()

		select {
		case <-ctx.Done():
			client.logger.Error("context cancelled while stopping jobs on peer", "address", peer.NodeAddress)
			return ctx.Err()
		default:
			scheduler0Secrets := client.scheduler0Secrets.GetSecrets()
			payload := models.FetchRemoteData{
				RequestId:    "stop_jobs",
				AuthUsername: scheduler0Secrets.AuthUsername,
				AuthPassword: scheduler0Secrets.AuthPassword,
			}
			_, writeErr := payload.WriteTo(conn)
			if writeErr != nil {
				client.logger.Error("failed to write stop jobs command to peer", "error", writeErr, "address", peer.NodeAddress)
				return writeErr
			}

			// Read response to confirm command was received
			res, decodeErr := decode(conn)
			if decodeErr != nil {
				client.logger.Error("failed to decode response from peer", "error", decodeErr, "address", peer.NodeAddress)
				return decodeErr
			}

			response, ok := res.(*models.String)
			if ok {
				if string(*response) == "incorrect_credentials" {
					client.logger.Error("authentication failed when stopping jobs on peer", "address", peer.NodeAddress)
					// Authentication failures are not retriable by design.
					return fmt.Errorf("authentication failed")
				}
				client.logger.Info("successfully sent stop jobs command to peer", "address", peer.NodeAddress, "response", string(*response))
				return nil
			}

			client.logger.Warn("unexpected response type from peer when stopping jobs", "address", peer.NodeAddress, "type", fmt.Sprintf("%T", res))
			return nil
		}
	})
}

func (client tcpClient) StartJobs(ctx context.Context, node *nodeService, peer config.RaftNode) error {
	return client.withRetry(ctx, "StartJobs", func() error {
		configs := client.scheduler0Configs.GetConfigurations()
		conn, err := client.ln.Dial(peer.NodeAddress, time.Duration(configs.RaftTransportTimeout)*time.Second)
		if err != nil {
			client.logger.Error("failed to open connection to peer node", "error", err, "address", peer.NodeAddress)
			return err
		}
		defer conn.Close()

		select {
		case <-ctx.Done():
			client.logger.Error("context cancelled while starting jobs on peer", "address", peer.NodeAddress)
			return ctx.Err()
		default:
			scheduler0Secrets := client.scheduler0Secrets.GetSecrets()
			payload := models.FetchRemoteData{
				RequestId:    "start_jobs",
				AuthUsername: scheduler0Secrets.AuthUsername,
				AuthPassword: scheduler0Secrets.AuthPassword,
			}
			_, writeErr := payload.WriteTo(conn)
			if writeErr != nil {
				client.logger.Error("failed to write start jobs command to peer", "error", writeErr, "address", peer.NodeAddress)
				return writeErr
			}

			// Read response to confirm command was received
			res, decodeErr := decode(conn)
			if decodeErr != nil {
				client.logger.Error("failed to decode response from peer", "error", decodeErr, "address", peer.NodeAddress)
				return decodeErr
			}

			response, ok := res.(*models.String)
			if ok {
				if string(*response) == "incorrect_credentials" {
					client.logger.Error("authentication failed when starting jobs on peer", "address", peer.NodeAddress)
					// Authentication failures are not retriable by design.
					return fmt.Errorf("authentication failed")
				}
				client.logger.Info("successfully sent start jobs command to peer", "address", peer.NodeAddress, "response", string(*response))
				return nil
			}

			client.logger.Warn("unexpected response type from peer when starting jobs", "address", peer.NodeAddress, "type", fmt.Sprintf("%T", res))
			return nil
		}
	})
}

func NewTCPClient(logger hclog.Logger, ln network.Listener, scheduler0Configs config.Scheduler0Config, scheduler0Secrets secrets.Scheduler0Secrets) Client {
	return tcpClient{
		logger:            logger.Named("node-tcp-client"),
		scheduler0Configs: scheduler0Configs,
		scheduler0Secrets: scheduler0Secrets,
		ln:                ln,
	}
}

func decode(r io.Reader) (models.NodeTCPPayload, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return nil, err
	}
	var payload models.NodeTCPPayload
	switch typ {
	case models.NodeAuthPayload:
		payload = new(models.NodeAuth)
	case models.StringPayload:
		payload = new(models.String)
	case models.FetchRemoteDataPayload:
		payload = new(models.FetchRemoteData)
	case models.AsyncTaskPayload:
		payload = new(models.AsyncTask)
	case models.QuotaAllocationPayload:
		payload = new(models.QuotaAllocation)
	case models.LocalQuotaRequestPayload:
		payload = new(models.LocalQuotaRequest)
	case models.LocalQuotaResponsePayload:
		payload = new(models.LocalQuotaResponse)
	case models.AccountExhaustionPayload:
		payload = new(models.AccountExhaustion)
	case models.JobUpdateRequestPayload:
		payload = new(models.JobUpdateRequest)
	default:
		return nil, errors.New("unknown type")
	}
	_, err = payload.ReadFrom(io.MultiReader(bytes.NewReader([]byte{typ}), r))
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (client tcpClient) SendQuotaAllocation(ctx context.Context, peer config.RaftNode, accountAllocations map[uint64]uint64) error {
	return client.withRetry(ctx, "SendQuotaAllocation", func() error {
		configs := client.scheduler0Configs.GetConfigurations()
		conn, err := client.ln.Dial(peer.NodeAddress, time.Duration(configs.RaftTransportTimeout)*time.Second)
		if err != nil {
			client.logger.Error("failed to open connection to peer node", "error", err, "address", peer.NodeAddress)
			return err
		}
		defer conn.Close()

		select {
		case <-ctx.Done():
			client.logger.Error("context cancelled while sending quota allocation to peer", "address", peer.NodeAddress)
			return ctx.Err()
		default:
			scheduler0Secrets := client.scheduler0Secrets.GetSecrets()
			payload := models.QuotaAllocation{
				AuthUsername:       scheduler0Secrets.AuthUsername,
				AuthPassword:       scheduler0Secrets.AuthPassword,
				AccountAllocations: accountAllocations,
			}
			_, writeErr := payload.WriteTo(conn)
			if writeErr != nil {
				client.logger.Error("failed to write quota allocation to peer", "error", writeErr, "address", peer.NodeAddress)
				return writeErr
			}

			// Read response to confirm allocation was received
			res, decodeErr := decode(conn)
			if decodeErr != nil {
				client.logger.Error("failed to decode response from peer", "error", decodeErr, "address", peer.NodeAddress)
				return decodeErr
			}

			response, ok := res.(*models.String)
			if ok {
				responseStr := string(*response)
				if responseStr == "incorrect_credentials" {
					client.logger.Error("authentication failed when sending quota allocation to peer", "address", peer.NodeAddress)
					return fmt.Errorf("authentication failed")
				}
				if strings.HasPrefix(responseStr, "error:") {
					client.logger.Error("quota allocation failed on peer", "address", peer.NodeAddress, "error", responseStr)
					return fmt.Errorf("quota allocation failed: %s", responseStr)
				}
				client.logger.Info("successfully sent quota allocation to peer", "address", peer.NodeAddress, "response", responseStr, "accountCount", len(accountAllocations))
				return nil
			}

			client.logger.Warn("unexpected response type from peer when sending quota allocation", "address", peer.NodeAddress, "type", fmt.Sprintf("%T", res))
			return nil
		}
	})
}

func (client tcpClient) RequestLocalQuotaAllocations(ctx context.Context, peer config.RaftNode) (map[uint64]uint64, error) {
	var allocations map[uint64]uint64
	err := client.withRetry(ctx, "RequestLocalQuotaAllocations", func() error {
		configs := client.scheduler0Configs.GetConfigurations()
		conn, err := client.ln.Dial(peer.NodeAddress, time.Duration(configs.RaftTransportTimeout)*time.Second)
		if err != nil {
			client.logger.Error("failed to open connection to peer node", "error", err, "address", peer.NodeAddress)
			return err
		}
		defer conn.Close()

		select {
		case <-ctx.Done():
			client.logger.Error("context cancelled while requesting local quota from peer", "address", peer.NodeAddress)
			return ctx.Err()
		default:
			scheduler0Secrets := client.scheduler0Secrets.GetSecrets()
			payload := models.LocalQuotaRequest{
				AuthUsername: scheduler0Secrets.AuthUsername,
				AuthPassword: scheduler0Secrets.AuthPassword,
			}
			_, writeErr := payload.WriteTo(conn)
			if writeErr != nil {
				client.logger.Error("failed to write local quota request to peer", "error", writeErr, "address", peer.NodeAddress)
				return writeErr
			}

			res, decodeErr := decode(conn)
			if decodeErr != nil {
				client.logger.Error("failed to decode response from peer for local quota request", "error", decodeErr, "address", peer.NodeAddress)
				return decodeErr
			}

			response, ok := res.(*models.LocalQuotaResponse)
			if ok {
				allocations = response.AccountAllocations
				client.logger.Info("successfully received local quota allocations from peer", "address", peer.NodeAddress, "accountCount", len(allocations))
				return nil
			}

			client.logger.Warn("unexpected response type from peer for local quota request", "address", peer.NodeAddress, "type", fmt.Sprintf("%T", res))
			return fmt.Errorf("unexpected response type from peer")
		}
	})
	return allocations, err
}

func (client tcpClient) NotifyAccountExhaustion(ctx context.Context, leader config.RaftNode, accountId uint64) error {
	return client.withRetry(ctx, "NotifyAccountExhaustion", func() error {
		configs := client.scheduler0Configs.GetConfigurations()
		conn, err := client.ln.Dial(leader.NodeAddress, time.Duration(configs.RaftTransportTimeout)*time.Second)
		if err != nil {
			client.logger.Error("failed to open connection to leader node", "error", err, "address", leader.NodeAddress)
			return err
		}
		defer conn.Close()

		select {
		case <-ctx.Done():
			client.logger.Error("context cancelled while notifying account exhaustion to leader", "address", leader.NodeAddress)
			return ctx.Err()
		default:
			scheduler0Secrets := client.scheduler0Secrets.GetSecrets()
			payload := models.AccountExhaustion{
				AuthUsername: scheduler0Secrets.AuthUsername,
				AuthPassword: scheduler0Secrets.AuthPassword,
				AccountId:    accountId,
			}
			_, writeErr := payload.WriteTo(conn)
			if writeErr != nil {
				client.logger.Error("failed to write account exhaustion notification to leader", "error", writeErr, "address", leader.NodeAddress, "accountId", accountId)
				return writeErr
			}

			// Read response to confirm notification was received
			res, decodeErr := decode(conn)
			if decodeErr != nil {
				client.logger.Error("failed to decode response from leader for account exhaustion notification", "error", decodeErr, "address", leader.NodeAddress)
				return decodeErr
			}

			response, ok := res.(*models.String)
			if ok {
				responseStr := string(*response)
				if responseStr == "incorrect_credentials" {
					client.logger.Error("authentication failed when notifying account exhaustion to leader", "address", leader.NodeAddress)
					return fmt.Errorf("authentication failed")
				}
				if strings.HasPrefix(responseStr, "error:") {
					client.logger.Error("account exhaustion notification failed on leader", "address", leader.NodeAddress, "error", responseStr)
					return fmt.Errorf("account exhaustion notification failed: %s", responseStr)
				}
				client.logger.Info("successfully notified account exhaustion to leader", "address", leader.NodeAddress, "response", responseStr, "accountId", accountId)
				return nil
			}

			client.logger.Warn("unexpected response type from leader when notifying account exhaustion", "address", leader.NodeAddress, "type", fmt.Sprintf("%T", res))
			return nil
		}
	})
}

func (client tcpClient) UpdateJobOnLeader(ctx context.Context, leader config.RaftNode, job models.Job) error {
	return client.withRetry(ctx, "UpdateJobOnLeader", func() error {
		configs := client.scheduler0Configs.GetConfigurations()
		conn, err := client.ln.Dial(leader.NodeAddress, time.Duration(configs.RaftTransportTimeout)*time.Second)
		if err != nil {
			client.logger.Error("failed to open connection to leader node", "error", err, "address", leader.NodeAddress)
			return err
		}
		defer conn.Close()

		select {
		case <-ctx.Done():
			client.logger.Error("context cancelled while updating job on leader", "address", leader.NodeAddress, "jobId", job.ID)
			return ctx.Err()
		default:
			scheduler0Secrets := client.scheduler0Secrets.GetSecrets()
			payload := models.JobUpdateRequest{
				AuthUsername: scheduler0Secrets.AuthUsername,
				AuthPassword: scheduler0Secrets.AuthPassword,
				Job:          job,
			}
			_, writeErr := payload.WriteTo(conn)
			if writeErr != nil {
				client.logger.Error("failed to write job update request to leader", "error", writeErr, "address", leader.NodeAddress, "jobId", job.ID)
				return writeErr
			}

			// Read response to confirm update was received
			res, decodeErr := decode(conn)
			if decodeErr != nil {
				client.logger.Error("failed to decode response from leader for job update", "error", decodeErr, "address", leader.NodeAddress)
				return decodeErr
			}

			response, ok := res.(*models.String)
			if ok {
				responseStr := string(*response)
				if responseStr == "incorrect_credentials" {
					client.logger.Error("authentication failed when updating job on leader", "address", leader.NodeAddress)
					return fmt.Errorf("authentication failed")
				}
				if strings.HasPrefix(responseStr, "error:") {
					client.logger.Error("job update failed on leader", "address", leader.NodeAddress, "error", responseStr, "jobId", job.ID)
					return fmt.Errorf("job update failed: %s", responseStr)
				}
				client.logger.Info("successfully updated job on leader", "address", leader.NodeAddress, "response", responseStr, "jobId", job.ID)
				return nil
			}

			client.logger.Warn("unexpected response type from leader when updating job", "address", leader.NodeAddress, "type", fmt.Sprintf("%T", res))
			return nil
		}
	})
}

func (client tcpClient) FetchUncommittedLogsFromPeersPhase1(ctx context.Context, node *nodeService, peerFanIns []models.PeerFanIn) {
	client.logger.Info("fetching uncommitted logs from peers phase 1", "peerFanIns", len(peerFanIns))
	for _, peerFanIn := range peerFanIns {
		err := client.withRetry(ctx, "FetchUncommittedLogsFromPeersPhase1", func() error {
			configs := client.scheduler0Configs.GetConfigurations()
			conn, err := client.ln.Dial(peerFanIn.PeerNodeAddress, time.Duration(configs.RaftTransportTimeout)*time.Second)
			if err != nil {
				client.logger.Error("failed to open-connection to connect to node", "error", err.Error())
				return err
			}
			defer conn.Close()

			select {
			case <-ctx.Done():
				client.logger.Error("closed connection due to context done event")
				return ctx.Err()
			default:
				scheduler0Secrets := client.scheduler0Secrets.GetSecrets()
				payload := models.FetchRemoteData{
					AuthUsername: scheduler0Secrets.AuthUsername,
					AuthPassword: scheduler0Secrets.AuthPassword,
				}
				_, writeErr := payload.WriteTo(conn)
				if writeErr != nil {
					client.logger.Error("failed to write to connection", "error", writeErr.Error())
					return writeErr
				}

				for {
					client.logger.Info("fetching uncommitted logs from peers phase 1", "peerFanIn", peerFanIn.PeerNodeAddress)
					res, decodeErr := decode(conn)
					if decodeErr != nil {
						client.logger.Error("cannot decode response from server", "error", decodeErr.Error())
						return decodeErr
					}
					requestId, ok := res.(*models.String)
					if ok {
						if string(*requestId) == "incorrect_credentials" {
							client.logger.Error("fetching failed due to incorrect_credentials")
							// Auth failure: do not retry.
							return fmt.Errorf("incorrect_credentials")
						}
						client.logger.Info("successfully fetched request id", "requestId", string(*requestId))
						peerFanIn.RequestId = string(*requestId)
						peerFanIn.State = models.PeerFanInStateGetRequestId
						node.fanIns.Store(peerFanIn.PeerNodeAddress, peerFanIn)
						return nil
					}

					client.logger.Error("could not parse response", "response", payload, "but expected a string response")
					// Protocol-level issue, not obviously retriable.
					return fmt.Errorf("unexpected response type when fetching uncommitted logs phase 1")
				}
			}
		})

		if err != nil {
			client.logger.Error("failed to fetch uncommitted logs from peer after retries (phase 1)", "peerNodeAddress", peerFanIn.PeerNodeAddress, "error", err)
			return
		}
	}
}

func (client tcpClient) FetchUncommittedLogsFromPeersPhase2(ctx context.Context, node *nodeService, peerFanIns []models.PeerFanIn) {
	client.logger.Info("fetching uncommitted logs from peers phase 2", "peerFanIns", len(peerFanIns))
	for _, peerFanIn := range peerFanIns {
		err := client.withRetry(ctx, "FetchUncommittedLogsFromPeersPhase2", func() error {
			configs := client.scheduler0Configs.GetConfigurations()
			conn, err := client.ln.Dial(peerFanIn.PeerNodeAddress, time.Duration(configs.RaftTransportTimeout)*time.Second)
			if err != nil {
				client.logger.Error("failed to open-connection to connect to node", "error", err.Error())
				return err
			}
			defer conn.Close()

			select {
			case <-ctx.Done():
				client.logger.Error("closed connection due to context done event")
				return ctx.Err()
			default:
				scheduler0Secrets := client.scheduler0Secrets.GetSecrets()
				payload := models.FetchRemoteData{
					RequestId:    peerFanIn.RequestId,
					AuthUsername: scheduler0Secrets.AuthUsername,
					AuthPassword: scheduler0Secrets.AuthPassword,
				}
				_, writeErr := payload.WriteTo(conn)
				if writeErr != nil {
					client.logger.Error("failed to write to connection", "error", writeErr.Error())
					return writeErr
				}
				for {
					client.logger.Info("fetching uncommitted logs from peers phase 2", "peerFanIn", peerFanIn.PeerNodeAddress)
					res, decodeErr := decode(conn)
					if decodeErr != nil {
						client.logger.Error("cannot decode response from server", "error", decodeErr.Error())
						return decodeErr
					}
					asyncTask, ok := res.(*models.AsyncTask)
					if ok {
						if (*asyncTask).RequestId == "" {
							client.logger.Error("invalid async task returned")
							// Treat as non-retriable protocol error.
							return fmt.Errorf("invalid async task with empty RequestId")
						}
						client.logger.Info("successfully fetched async task", "asyncTask", (*asyncTask).RequestId)
						var localData models.LocalData
						marshalErr := json.Unmarshal([]byte((*asyncTask).Output), &localData)
						if marshalErr != nil {
							node.logger.Error("failed to read uncommitted execution logs from", "node address", peerFanIn.PeerNodeAddress, "error", marshalErr.Error())
							// Local unmarshal error is not retriable.
							return marshalErr
						}

						peerFanIn.Data = localData
						peerFanIn.State = models.PeerFanInStateGetExecutionsLogs
						node.fanIns.Store(peerFanIn.PeerNodeAddress, peerFanIn)
						node.logger.Info("successfully fetch execution logs from", "node address", peerFanIn.PeerNodeAddress)
						return nil
					}

					client.logger.Error("could not parse response", "response", payload, "but expected a async task response", conn.RemoteAddr())
					// Protocol-level issue, not obviously retriable.
					return fmt.Errorf("unexpected response type when fetching uncommitted logs phase 2")
				}
			}
		})

		if err != nil {
			client.logger.Error("failed to fetch uncommitted logs from peer after retries (phase 2)", "peerNodeAddress", peerFanIn.PeerNodeAddress, "error", err)
			return
		}
	}
}

func (client tcpClient) ConnectNode(replica config.RaftNode) (*Status, error) {
	var status *Status

	// ConnectNode does not currently accept a context, so we use a background context
	// and rely on TCP timeouts and retry limits to bound the operation.
	ctx := context.Background()

	err := client.withRetry(ctx, "ConnectNode", func() error {
		configs := client.scheduler0Configs.GetConfigurations()
		conn, err := client.ln.Dial(replica.NodeAddress, time.Duration(configs.RaftTransportTimeout)*time.Second)
		if err != nil {
			client.logger.Error("failed to open-connection to connect to node", "error", err.Error())
			return err
		}
		defer conn.Close()

		scheduler0Secrets := client.scheduler0Secrets.GetSecrets()

		nodeAuth := models.NodeAuth{
			AuthUsername: scheduler0Secrets.AuthUsername,
			AuthPassword: scheduler0Secrets.AuthPassword,
		}

		_, err = nodeAuth.WriteTo(conn)
		if err != nil {
			client.logger.Error("failed to write to connection", "error", err.Error())
			return err
		}

		s := Status{}
		for {
			payload, decodeErr := decode(conn)
			if decodeErr != nil {
				return decodeErr
			}
			connected, ok := payload.(*models.String)

			if ok {
				if *connected != "incorrect_credentials" {
					s.IsAuth = true
					s.IsAlive = true
					status = &s
					return nil
				}

				client.logger.Error("could not authenticate with", "replica", replica.NodeId)
				// Authentication failure is not retriable.
				return fmt.Errorf("authentication failed with replica %d", replica.NodeId)
			}

			client.logger.Error("could not parse response", "response", payload)
			// Unexpected payload type: not clearly retriable.
			return fmt.Errorf("unexpected response type when connecting to node")
		}
	})

	if err != nil {
		return nil, err
	}

	return status, nil
}
