package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"scheduler0-private/pkg/models"
	async_task_service "scheduler0-private/pkg/service/async_task"
	"scheduler0-private/pkg/service/node"
	"scheduler0-private/pkg/utils"
	"strconv"
	"time"
)

type ClusterController interface {
	RemoveSelf(w http.ResponseWriter, r *http.Request)
	AddSelf(w http.ResponseWriter, r *http.Request)
	ForceRebuild(w http.ResponseWriter, r *http.Request)
	ResetRaft(w http.ResponseWriter, r *http.Request)
	RemoveNode(w http.ResponseWriter, r *http.Request)
	AddNode(w http.ResponseWriter, r *http.Request)
	PromoteNode(w http.ResponseWriter, r *http.Request)
	DemoteNode(w http.ResponseWriter, r *http.Request)
	TransferLeadership(w http.ResponseWriter, r *http.Request)
	ListNodes(w http.ResponseWriter, r *http.Request)
	DumpScheduleQueue(w http.ResponseWriter, r *http.Request)
	DumpJobExecutionsCache(w http.ResponseWriter, r *http.Request)
	DumpJobQueues(w http.ResponseWriter, r *http.Request)
	DumpJobQueueVersions(w http.ResponseWriter, r *http.Request)
	BackupDatabase(w http.ResponseWriter, r *http.Request)
	RestoreDatabase(w http.ResponseWriter, r *http.Request)
}

type clusterController struct {
	service          node.NodeService
	logger           *log.Logger
	asyncTaskService async_task_service.AsyncTaskService
}

func NewClusterController(logger *log.Logger, service node.NodeService, asyncTaskService async_task_service.AsyncTaskService) ClusterController {
	return &clusterController{
		service:          service,
		logger:           logger,
		asyncTaskService: asyncTaskService,
	}
}

// RemoveSelf removes this node from Raft membership and unregisters it from etcd.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) RemoveSelf(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RemoveSelf entry", r.URL.Path))

	if err := c.service.RemoveSelfFromCluster(r.Context()); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RemoveSelf error: failed to remove self from cluster, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RemoveSelf success, status=200", r.URL.Path))
	utils.SendJSON(w, map[string]string{"status": "removed"}, true, http.StatusOK, nil)
}

// AddSelf ensures this node is registered in etcd and part of the Raft cluster.
func (c *clusterController) AddSelf(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddSelf entry", r.URL.Path))

	if err := c.service.AddSelfToCluster(r.Context()); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddSelf error: failed to add self to cluster, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddSelf success, status=200", r.URL.Path))
	utils.SendJSON(w, map[string]string{"status": "added"}, true, http.StatusOK, nil)
}

// ForceRebuild forces a rebuild of the Raft cluster. This should only be called on the seed node.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) ForceRebuild(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ForceRebuild entry, query=%s", r.URL.Path, r.URL.RawQuery))

	// Get seedNodeId from query parameter
	seedNodeIdStr := r.URL.Query().Get("seedNodeId")
	if seedNodeIdStr == "" {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ForceRebuild error: seedNodeId query parameter is required", r.URL.Path))
		utils.SendJSON(w, "seedNodeId query parameter is required", false, http.StatusBadRequest, nil)
		return
	}

	seedNodeId, err := strconv.ParseUint(seedNodeIdStr, 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ForceRebuild error: invalid seedNodeId, seedNodeId=%s, error=%v", r.URL.Path, seedNodeIdStr, err))
		utils.SendJSON(w, "invalid seedNodeId: "+err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ForceRebuild processing, seedNodeId=%d", r.URL.Path, seedNodeId))
	if err := c.service.ForceRebuildCluster(r.Context(), seedNodeId); err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ForceRebuild error: failed to force rebuild cluster, seedNodeId=%d, error=%v", r.URL.Path, seedNodeId, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ForceRebuild success, status=200, seedNodeId=%d", r.URL.Path, seedNodeId))
	utils.SendJSON(w, map[string]string{"status": "rebuild initiated"}, true, http.StatusOK, nil)
}

// ResetRaft clears local Raft state on this node and exits the process.
// The HTTP response must be sent and flushed before the process exits.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) ResetRaft(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ResetRaft entry", r.URL.Path))

	// Send response immediately and flush to ensure it's sent before process exits
	utils.SendJSON(w, map[string]string{"status": "raft reset"}, true, http.StatusOK, nil)

	// Flush the response to ensure it's sent before the process exits
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ResetRaft processing, response sent, initiating raft reset", r.URL.Path))

	// Small delay to ensure response is fully sent
	time.Sleep(100 * time.Millisecond)

	// Call ResetRaftState which will exit the process after deleting files
	if err := c.service.ResetRaftState(r.Context()); err != nil {
		// This should not be reached if ResetRaftState exits successfully,
		// but handle it just in case
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - ResetRaft error: failed to reset raft state, error=%v", r.URL.Path, err))
		// Process will exit anyway, so we can't send another response
	}
}

// RemoveNode removes a node from the Raft cluster. Only the leader can perform this operation.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) RemoveNode(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RemoveNode entry, query=%s", r.URL.Path, r.URL.RawQuery))

	// Get nodeId from query parameter
	nodeIdStr := r.URL.Query().Get("nodeId")
	if nodeIdStr == "" {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RemoveNode error: nodeId query parameter is required", r.URL.Path))
		utils.SendJSON(w, "nodeId query parameter is required", false, http.StatusBadRequest, nil)
		return
	}

	nodeId, err := strconv.ParseUint(nodeIdStr, 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RemoveNode error: invalid nodeId, nodeId=%s, error=%v", r.URL.Path, nodeIdStr, err))
		utils.SendJSON(w, "invalid nodeId: "+err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RemoveNode processing, nodeId=%d", r.URL.Path, nodeId))
	if err := c.service.RemoveNode(r.Context(), nodeId); err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "node is not leader; cannot remove node" {
			statusCode = http.StatusForbidden
		}
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RemoveNode error: failed to remove node, nodeId=%d, error=%v", r.URL.Path, nodeId, err))
		utils.SendJSON(w, err.Error(), false, statusCode, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RemoveNode success, status=200, nodeId=%d", r.URL.Path, nodeId))
	utils.SendJSON(w, map[string]string{"status": "node removed"}, true, http.StatusOK, nil)
}

// AddNode adds a node to the Raft cluster. Only the leader can perform this operation.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) AddNode(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddNode entry, query=%s", r.URL.Path, r.URL.RawQuery))

	// Get nodeId and nodeAddress from query parameters
	nodeIdStr := r.URL.Query().Get("nodeId")
	nodeAddress := r.URL.Query().Get("nodeAddress")
	clientAddress := r.URL.Query().Get("clientAddress")

	if nodeIdStr == "" {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddNode error: nodeId query parameter is required", r.URL.Path))
		utils.SendJSON(w, "nodeId query parameter is required", false, http.StatusBadRequest, nil)
		return
	}

	if nodeAddress == "" {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddNode error: nodeAddress query parameter is required", r.URL.Path))
		utils.SendJSON(w, "nodeAddress query parameter is required", false, http.StatusBadRequest, nil)
		return
	}

	if clientAddress == "" {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddClientAddress query parameter is required", r.URL.Path))
		utils.SendJSON(w, "clientAddress query parameter is required", false, http.StatusBadRequest, nil)
		return
	}

	nodeId, err := strconv.ParseUint(nodeIdStr, 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddNode error: invalid nodeId, nodeId=%s, error=%v", r.URL.Path, nodeIdStr, err))
		utils.SendJSON(w, "invalid nodeId: "+err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddNode processing, nodeId=%d, nodeAddress=%s", r.URL.Path, nodeId, nodeAddress))
	if err := c.service.AddNode(r.Context(), nodeId, nodeAddress, clientAddress); err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "node is not leader; cannot add node" {
			statusCode = http.StatusForbidden
		}
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddNode error: failed to add node, nodeId=%d, nodeAddress=%s, error=%v", r.URL.Path, nodeId, nodeAddress, err))
		utils.SendJSON(w, err.Error(), false, statusCode, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - AddNode success, status=200, nodeId=%d, nodeAddress=%s", r.URL.Path, nodeId, nodeAddress))
	utils.SendJSON(w, map[string]string{"status": "node added"}, true, http.StatusOK, nil)
}

// PromoteNode promotes a non-voter node to a voter in the Raft cluster. Only the leader can perform this operation.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) PromoteNode(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - PromoteNode entry, query=%s", r.URL.Path, r.URL.RawQuery))

	// Get nodeId from query parameter
	nodeIdStr := r.URL.Query().Get("nodeId")
	if nodeIdStr == "" {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - PromoteNode error: nodeId query parameter is required", r.URL.Path))
		utils.SendJSON(w, "nodeId query parameter is required", false, http.StatusBadRequest, nil)
		return
	}

	nodeId, err := strconv.ParseUint(nodeIdStr, 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - PromoteNode error: invalid nodeId, nodeId=%s, error=%v", r.URL.Path, nodeIdStr, err))
		utils.SendJSON(w, "invalid nodeId: "+err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - PromoteNode processing, nodeId=%d", r.URL.Path, nodeId))
	if err := c.service.PromoteNode(r.Context(), nodeId); err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "node is not leader; cannot promote node" {
			statusCode = http.StatusForbidden
		}
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - PromoteNode error: failed to promote node, nodeId=%d, error=%v", r.URL.Path, nodeId, err))
		utils.SendJSON(w, err.Error(), false, statusCode, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - PromoteNode success, status=200, nodeId=%d", r.URL.Path, nodeId))
	utils.SendJSON(w, map[string]string{"status": "node promoted to voter"}, true, http.StatusOK, nil)
}

// DemoteNode demotes a voter node to a non-voter in the Raft cluster. Only the leader can perform this operation.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) DemoteNode(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - DemoteNode entry, query=%s", r.URL.Path, r.URL.RawQuery))

	// Get nodeId from query parameter
	nodeIdStr := r.URL.Query().Get("nodeId")
	if nodeIdStr == "" {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - DemoteNode error: nodeId query parameter is required", r.URL.Path))
		utils.SendJSON(w, "nodeId query parameter is required", false, http.StatusBadRequest, nil)
		return
	}

	nodeId, err := strconv.ParseUint(nodeIdStr, 10, 64)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - DemoteNode error: invalid nodeId, nodeId=%s, error=%v", r.URL.Path, nodeIdStr, err))
		utils.SendJSON(w, "invalid nodeId: "+err.Error(), false, http.StatusBadRequest, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - DemoteNode processing, nodeId=%d", r.URL.Path, nodeId))
	if err := c.service.DemoteNode(r.Context(), nodeId); err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "node is not leader; cannot demote node" {
			statusCode = http.StatusForbidden
		}
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - DemoteNode error: failed to demote node, nodeId=%d, error=%v", r.URL.Path, nodeId, err))
		utils.SendJSON(w, err.Error(), false, statusCode, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - DemoteNode success, status=200, nodeId=%d", r.URL.Path, nodeId))
	utils.SendJSON(w, map[string]string{"status": "node demoted to non-voter"}, true, http.StatusOK, nil)
}

// TransferLeadership transfers leadership to another node. Only the leader can perform this operation.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) TransferLeadership(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - TransferLeadership entry, query=%s", r.URL.Path, r.URL.RawQuery))

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - TransferLeadership processing", r.URL.Path))
	if err := c.service.TransferLeadership(r.Context()); err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "node is not leader; cannot transfer leadership" {
			statusCode = http.StatusForbidden
		}
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - TransferLeadership error: failed to transfer leadership, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, statusCode, nil)
		return
	}
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - TransferLeadership success, status=200", r.URL.Path))
	utils.SendJSON(w, map[string]string{"status": "leadership transferred"}, true, http.StatusOK, nil)
}

// ListNodes returns a list of all nodes in the Raft cluster.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) ListNodes(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - ListNodes entry", r.URL.Path))

	nodes, err := c.service.ListNodes(r.Context())
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - ListNodes error: failed to list nodes, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - ListNodes success, status=200, nodeCount=%d", r.URL.Path, len(nodes)))
	utils.SendJSON(w, nodes, true, http.StatusOK, nil)
}

// DumpScheduleQueue returns the schedule queue from the executor service.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) DumpScheduleQueue(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpScheduleQueue entry", r.URL.Path))

	jobExecutor := c.service.GetJobExecutor()
	if jobExecutor == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpScheduleQueue error: job executor not available", r.URL.Path))
		utils.SendJSON(w, "job executor not available", false, http.StatusInternalServerError, nil)
		return
	}

	scheduleQueue := jobExecutor.GetScheduleQueue()
	if scheduleQueue == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpScheduleQueue error: schedule queue not available", r.URL.Path))
		utils.SendJSON(w, "schedule queue not available", false, http.StatusInternalServerError, nil)
		return
	}

	// Extract all items from the queue
	items := scheduleQueue.GetAllItems()
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpScheduleQueue success, status=200, itemCount=%d", r.URL.Path, len(items)))
	utils.SendJSON(w, items, true, http.StatusOK, nil)
}

// DumpJobExecutionsCache returns the job executions cache from the executor service.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) DumpJobExecutionsCache(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpJobExecutionsCache entry", r.URL.Path))

	jobExecutor := c.service.GetJobExecutor()
	if jobExecutor == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpJobExecutionsCache error: job executor not available", r.URL.Path))
		utils.SendJSON(w, "job executor not available", false, http.StatusInternalServerError, nil)
		return
	}

	cache := jobExecutor.GetExecutionsCache()
	if cache == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpJobExecutionsCache error: cache not available", r.URL.Path))
		utils.SendJSON(w, "cache not available", false, http.StatusInternalServerError, nil)
		return
	}

	// Convert sync.Map to regular map for JSON serialization
	result := make(map[uint64]models.JobSchedule)
	cache.Range(func(key, value interface{}) bool {
		if jobId, ok := key.(uint64); ok {
			if schedule, ok := value.(models.JobSchedule); ok {
				result[jobId] = schedule
			}
		}
		return true
	})

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpJobExecutionsCache success, status=200, cacheSize=%d", r.URL.Path, len(result)))
	utils.SendJSON(w, result, true, http.StatusOK, nil)
}

// DumpJobQueues returns all job queues from the repository.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) DumpJobQueues(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpJobQueues entry", r.URL.Path))

	jobQueuesRepo := c.service.GetJobQueuesRepo()
	if jobQueuesRepo == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpJobQueues error: job queues repo not available", r.URL.Path))
		utils.SendJSON(w, "job queues repo not available", false, http.StatusInternalServerError, nil)
		return
	}

	queues, err := jobQueuesRepo.GetAllJobQueues()
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpJobQueues error: failed to get job queues, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpJobQueues success, status=200, queueCount=%d", r.URL.Path, len(queues)))
	utils.SendJSON(w, queues, true, http.StatusOK, nil)
}

// DumpJobQueueVersions returns all job queue versions from the repository.
// Auth is handled by existing middleware (peer/basic auth).
func (c *clusterController) DumpJobQueueVersions(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpJobQueueVersions entry", r.URL.Path))

	jobQueuesRepo := c.service.GetJobQueuesRepo()
	if jobQueuesRepo == nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpJobQueueVersions error: job queues repo not available", r.URL.Path))
		utils.SendJSON(w, "job queues repo not available", false, http.StatusInternalServerError, nil)
		return
	}

	versions, err := jobQueuesRepo.GetAllJobQueueVersions()
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpJobQueueVersions error: failed to get job queue versions, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("GET %s - DumpJobQueueVersions success, status=200, versionCount=%d", r.URL.Path, len(versions)))
	utils.SendJSON(w, versions, true, http.StatusOK, nil)
}

// BackupDatabase initiates an automatic timestamped backup
func (c *clusterController) BackupDatabase(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - BackupDatabase entry", r.URL.Path))

	_, err := c.asyncTaskService.AddTasks("backup", requestID, "backup-database", 1)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - BackupDatabase error: failed to add backup task, error=%v", r.URL.Path, err))
		utils.SendJSON(w, err.Error(), false, http.StatusInternalServerError, nil)
		return
	}

	if err := c.service.BackupDatabase(context.Background(), requestID); err != nil {
		c.logger.Printf("Backup failed: %v", err)
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - BackupDatabase initiated", r.URL.Path))
	utils.SendJSON(w, map[string]string{"status": "backup initiated", "requestId": requestID}, true, http.StatusAccepted, nil)
}

// RestoreDatabase restores from backup
func (c *clusterController) RestoreDatabase(w http.ResponseWriter, r *http.Request) {
	requestID := utils.GetRequestID(r.Context())
	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RestoreDatabase entry", r.URL.Path))

	var req struct {
		FilePath string `json:"filePath"`
	}

	requestBody := utils.ExtractBody(w, r)
	err := json.Unmarshal(requestBody, &req)
	if err != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RestoreDatabase error: failed to parse request body, error=%v", r.URL.Path, err))
		utils.SendJSON(w, "invalid request body", false, http.StatusBadRequest, nil)
		return
	}

	if req.FilePath == "" {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RestoreDatabase error: filePath is required", r.URL.Path))
		utils.SendJSON(w, "filePath is required", false, http.StatusBadRequest, nil)
		return
	}

	_, addTaskErr := c.asyncTaskService.AddTasks("restore", requestID, "restore-database", 1)
	if addTaskErr != nil {
		utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RestoreDatabase error: failed to add restore task, error=%v", r.URL.Path, addTaskErr))
		utils.SendJSON(w, addTaskErr.Error(), false, http.StatusInternalServerError, nil)
		return
	}

	if err := c.service.RestoreDatabase(context.Background(), req.FilePath, requestID); err != nil {
		c.logger.Printf("Restore failed: %v", err)
	}

	utils.LogWithRequestID(c.logger, requestID, "", fmt.Sprintf("POST %s - RestoreDatabase initiated, file=%s", r.URL.Path, req.FilePath))
	utils.SendJSON(w, map[string]string{"status": "restore initiated", "requestId": requestID}, true, http.StatusAccepted, nil)
}
