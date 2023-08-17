package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"io"
	"net/http"
	"scheduler0/config"
	"scheduler0/constants/headers"
	"scheduler0/models"
	"scheduler0/secrets"
	"scheduler0/utils"
	"time"
)

type nodeHTTPClient struct {
	logger            hclog.Logger
	scheduler0Configs config.Scheduler0Config
	scheduler0Secrets secrets.Scheduler0Secrets
}

func NewHTTPClient(logger hclog.Logger, scheduler0Configs config.Scheduler0Config, scheduler0Secrets secrets.Scheduler0Secrets) NodeClient {
	return nodeHTTPClient{
		logger:            logger,
		scheduler0Configs: scheduler0Configs,
		scheduler0Secrets: scheduler0Secrets,
	}
}

func (client nodeHTTPClient) FetchUncommittedLogsFromPeersPhase1(ctx context.Context, node *Node, peerFanIns []models.PeerFanIn) {
	for _, peerFanIn := range peerFanIns {
		httpClient := &http.Client{}
		httpRequest, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%v/v1/execution-logs", peerFanIn.PeerHTTPAddress), nil)
		if reqErr != nil {
			node.logger.Error("failed to create request to execution logs from", "node address", peerFanIn.PeerHTTPAddress, "error", reqErr.Error())
			node.fanIns.Delete(peerFanIn.PeerHTTPAddress)
		} else {
			httpRequest.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
			httpRequest.Header.Set(headers.PeerAddressHeader, utils.GetServerHTTPAddress())
			secret := node.scheduler0Secrets.GetSecrets()
			httpRequest.SetBasicAuth(secret.AuthUsername, secret.AuthPassword)
			res, err := httpClient.Do(httpRequest)
			if err != nil {
				node.logger.Error("failed to get uncommitted execution logs from", "node address", peerFanIn.PeerHTTPAddress, "error", err.Error())
				node.fanIns.Delete(peerFanIn.PeerHTTPAddress)
			} else {
				if res.StatusCode == http.StatusAccepted {
					location := res.Header.Get("Location")
					peerFanIn.RequestId = location
					closeErr := res.Body.Close()
					if closeErr != nil {
						node.logger.Error("failed to close body", "error", closeErr.Error())
						node.fanIns.Delete(peerFanIn.PeerHTTPAddress)
						return
					}
					peerFanIn.State = models.PeerFanInStateGetRequestId
					node.fanIns.Store(peerFanIn.PeerHTTPAddress, peerFanIn)
					node.logger.Info("successfully fetch execution logs from", "node address", peerFanIn.PeerHTTPAddress)
				} else {
					node.logger.Error("failed to get uncommitted execution logs from", "node address", peerFanIn.PeerHTTPAddress, "state code", res.StatusCode)
					node.fanIns.Delete(peerFanIn.PeerHTTPAddress)
				}
			}
		}
	}
}

func (client nodeHTTPClient) FetchUncommittedLogsFromPeersPhase2(ctx context.Context, node *Node, peerFanIns []models.PeerFanIn) {
	for _, peerFanIn := range peerFanIns {
		httpClient := &http.Client{}
		httpRequest, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s%s", peerFanIn.PeerHTTPAddress, peerFanIn.RequestId), nil)
		if reqErr != nil {
			node.logger.Error("failed to create request to execution logs from", "node address", peerFanIn.PeerHTTPAddress, "error", reqErr.Error())
		} else {
			httpRequest.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
			httpRequest.Header.Set(headers.PeerAddressHeader, utils.GetServerHTTPAddress())
			secret := node.scheduler0Secrets.GetSecrets()
			httpRequest.SetBasicAuth(secret.AuthUsername, secret.AuthPassword)
			res, err := httpClient.Do(httpRequest)
			if err != nil {
				node.logger.Error("failed to get uncommitted execution logs from", "node address", peerFanIn.PeerHTTPAddress, "error", err.Error())
				node.fanIns.Delete(peerFanIn.PeerHTTPAddress)
			} else {
				if res.StatusCode == http.StatusOK {
					data, readErr := io.ReadAll(res.Body)
					if readErr != nil {
						node.logger.Error("failed to read uncommitted execution logs from", peerFanIn.PeerHTTPAddress, "error", readErr.Error())
						node.fanIns.Delete(peerFanIn.PeerHTTPAddress)
					} else {
						var asyncTaskRes models.AsyncTaskRes
						marshalErr := json.Unmarshal([]byte(data), &asyncTaskRes)
						if marshalErr != nil {
							node.fanIns.Delete(peerFanIn.PeerHTTPAddress)
							node.logger.Error("failed to read uncommitted execution logs from", "node address", peerFanIn.PeerHTTPAddress, "error", marshalErr.Error())
						} else {
							var localData models.LocalData
							marshalErr = json.Unmarshal([]byte(asyncTaskRes.Data.Output), &localData)
							if marshalErr != nil {
								node.fanIns.Delete(peerFanIn.PeerHTTPAddress)
								node.logger.Error("failed to read uncommitted execution logs from", "node address", peerFanIn.PeerHTTPAddress, "error", marshalErr.Error())
							} else {
								peerFanIn.Data = localData
								closeErr := res.Body.Close()
								if closeErr != nil {
									node.fanIns.Delete(peerFanIn.PeerHTTPAddress)
									node.logger.Error("failed to close body", "error", closeErr.Error())
								} else {
									peerFanIn.State = models.PeerFanInStateGetExecutionsLogs
									node.fanIns.Store(peerFanIn.PeerHTTPAddress, peerFanIn)
									node.logger.Info("successfully fetch execution logs from", "node address", peerFanIn.PeerHTTPAddress)
								}
							}
						}
					}
				} else {
					node.logger.Error("failed to get uncommitted execution logs from", "node address", peerFanIn.PeerHTTPAddress, "state code", res.StatusCode)
					node.fanIns.Delete(peerFanIn.PeerHTTPAddress)
				}
			}
		}
	}
}

func (client nodeHTTPClient) ConnectNode(rep config.RaftNode) (*Status, error) {
	configs := client.scheduler0Configs.GetConfigurations()
	httpClient := http.Client{
		Timeout: time.Duration(configs.PeerAuthRequestTimeoutMs) * time.Millisecond,
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v1/peer-handshake", rep.Address), nil)
	if err != nil {
		client.logger.Error("failed to create request", "error", err)
		return nil, err
	}
	req.Header.Set(headers.PeerHeader, headers.PeerHeaderValue)
	req.Header.Set(headers.PeerAddressHeader, utils.GetServerHTTPAddress())
	credentials := client.scheduler0Secrets.GetSecrets()
	req.SetBasicAuth(credentials.AuthUsername, credentials.AuthPassword)

	start := time.Now()

	resp, err := httpClient.Do(req)
	if err != nil {
		client.logger.Error("failed to send request", "error", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	connectionTime := time.Since(start)

	if resp.StatusCode == http.StatusOK {
		data, ioErr := io.ReadAll(resp.Body)
		if ioErr != nil {
			client.logger.Error("failed to response", "error:", ioErr.Error())
			return nil, ioErr
		}

		body := Response{}

		unMarshalErr := json.Unmarshal(data, &body)
		if unMarshalErr != nil {
			client.logger.Error("failed to unmarshal response ", "error", unMarshalErr.Error())
			return nil, unMarshalErr
		}

		client.logger.Info("successfully authenticated", "replica-address", rep.Address)

		return &Status{
			IsAlive:            true,
			IsAuth:             true,
			IsLeader:           body.Data.IsLeader,
			LastConnectionTime: connectionTime,
		}, nil
	}

	client.logger.Error("could not authenticate", "replica-address", rep.Address, " status code:", resp.StatusCode)

	if resp.StatusCode == http.StatusUnauthorized {
		return &Status{
			IsAlive:            true,
			IsAuth:             false,
			IsLeader:           false,
			LastConnectionTime: connectionTime,
		}, nil
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		return &Status{
			IsAlive:            false,
			IsAuth:             false,
			IsLeader:           false,
			LastConnectionTime: connectionTime,
		}, nil
	}

	return &Status{
		IsAlive:            false,
		IsAuth:             false,
		IsLeader:           false,
		LastConnectionTime: connectionTime,
	}, nil
}
