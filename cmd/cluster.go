package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"scheduler0-private/pkg/constants"
	"scheduler0-private/pkg/constants/headers"
	"scheduler0-private/pkg/secrets"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var ClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "manage cluster nodes",
	Long:  `Commands for managing cluster nodes (remove, add, transfer leadership)`,
}

var removeNodeIdFlag uint64
var addNodeIdFlag uint64
var nodeAddressFlag string
var clientAddressFlag string
var promoteNodeIdFlag uint64
var demoteNodeIdFlag uint64
var targetNodeIdFlag uint64
var seedNodeIdFlag uint64

// Helper function to make HTTP request to leader
func makeLeaderRequest(method, endpoint string, queryParams map[string]string, logger *log.Logger) error {
	secrets := secrets.NewScheduler0Secrets().GetSecrets()

	if secrets == nil {
		logger.Fatalln("Scheduler0 secrets have not been set. Run ./scheduler0 secrets init to setup your secrets.")
	}

	if secrets.BaseURL == "" || strings.TrimSpace(secrets.BaseURL) == "" {
		logger.Fatalln("Base URL is not set in secrets. Run ./scheduler0 secrets init to set the base URL.")
	}

	// Parse base URL
	baseURL, err := url.Parse(secrets.BaseURL)
	if err != nil {
		logger.Fatalln("invalid base URL format:", err)
	}

	fullURL := baseURL.String() + constants.APIV1Base + endpoint

	// Add query parameters
	if len(queryParams) > 0 {
		u, err := url.Parse(fullURL)
		if err != nil {
			logger.Fatalln("failed to parse URL:", err)
		}
		q := u.Query()
		for key, value := range queryParams {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
		fullURL = u.String()
	}

	// Create request
	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		logger.Fatalln("failed to create request:", err)
	}

	// Set authentication
	req.SetBasicAuth(secrets.AuthUsername, secrets.AuthPassword)
	req.Header.Add(headers.PeerHeader, headers.PeerHeaderCMDValue)
	req.Header.Add("Content-Type", "application/json")

	// Make request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		logger.Fatalln("failed to make request:", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		logger.Fatalln("failed to read response:", err)
	}

	if res.StatusCode >= 400 {
		logger.Fatalln("request failed:", string(body))
	}

	fmt.Println(string(body))
	return nil
}

// Helper function to make HTTP request to a specific node
func makeNodeRequest(nodeURL, method, endpoint string, logger *log.Logger) ([]byte, error) {
	secrets := secrets.NewScheduler0Secrets().GetSecrets()

	if secrets == nil {
		return nil, fmt.Errorf("Scheduler0 secrets have not been set. Run ./scheduler0 secrets init to setup your secrets")
	}

	// Ensure nodeURL has http:// or https:// prefix
	if !strings.HasPrefix(nodeURL, "http://") && !strings.HasPrefix(nodeURL, "https://") {
		nodeURL = "http://" + nodeURL
	}

	fullURL := nodeURL + constants.APIV1Base + endpoint

	// Create request
	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication
	req.SetBasicAuth(secrets.AuthUsername, secrets.AuthPassword)
	req.Header.Add(headers.PeerHeader, headers.PeerHeaderCMDValue)
	req.Header.Add("Content-Type", "application/json")

	// Make request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", res.StatusCode, string(body))
	}

	return body, nil
}

// Helper function to get list of nodes
func getNodesList(logger *log.Logger) ([]map[string]interface{}, error) {
	secrets := secrets.NewScheduler0Secrets().GetSecrets()

	if secrets == nil {
		return nil, fmt.Errorf("Scheduler0 secrets have not been set. Run ./scheduler0 secrets init to setup your secrets")
	}

	if secrets.BaseURL == "" || strings.TrimSpace(secrets.BaseURL) == "" {
		return nil, fmt.Errorf("Base URL is not set in secrets. Run ./scheduler0 secrets init to set the base URL")
	}

	body, err := makeNodeRequest(secrets.BaseURL, "GET", "/cluster/list-nodes", logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes list: %w", err)
	}

	var response struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		// Try direct array format
		var nodes []map[string]interface{}
		if err := json.Unmarshal(body, &nodes); err != nil {
			return nil, fmt.Errorf("failed to parse nodes list: %w", err)
		}
		return nodes, nil
	}

	return response.Data, nil
}

var removeNodeCmd = &cobra.Command{
	Use:   "remove-node",
	Short: "Remove a node from the cluster",
	Long:  `Remove a node from the Raft cluster. This command forwards the request to the leader.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

		if removeNodeIdFlag == 0 {
			logger.Fatalln("--node-id is required")
		}

		confirmPrompt := promptui.Prompt{
			Label:       fmt.Sprintf("WARNING: This will remove node %d from the Raft cluster. This may affect cluster availability. Are you sure you want to continue? [y/N]:", removeNodeIdFlag),
			HideEntered: false,
			Default:     "N",
		}
		confirm, err := confirmPrompt.Run()
		if err != nil {
			logger.Fatalln("Failed to read confirmation:", err)
		}

		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "y" && confirm != "yes" {
			logger.Println("Operation cancelled.")
			return
		}

		queryParams := map[string]string{
			"nodeId": fmt.Sprintf("%d", removeNodeIdFlag),
		}

		if err := makeLeaderRequest("POST", "/cluster/remove-node", queryParams, logger); err != nil {
			logger.Fatalln("failed to remove node:", err)
		}
	},
}

var addNodeCmd = &cobra.Command{
	Use:   "add-node",
	Short: "Add a node to the cluster",
	Long:  `Add a node to the Raft cluster. This command forwards the request to the leader.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

		if addNodeIdFlag == 0 {
			logger.Fatalln("--node-id is required")
		}

		if nodeAddressFlag == "" {
			logger.Fatalln("--node-address is required")
		}

		if clientAddressFlag == "" {
			logger.Fatalln("--client-address is required")
		}

		confirmPrompt := promptui.Prompt{
			Label:       fmt.Sprintf("WARNING: This will add node %d (%s) to the Raft cluster. Are you sure you want to continue? [y/N]:", addNodeIdFlag, nodeAddressFlag),
			HideEntered: false,
			Default:     "N",
		}
		confirm, err := confirmPrompt.Run()
		if err != nil {
			logger.Fatalln("Failed to read confirmation:", err)
		}

		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "y" && confirm != "yes" {
			logger.Println("Operation cancelled.")
			return
		}

		queryParams := map[string]string{
			"nodeId":        fmt.Sprintf("%d", addNodeIdFlag),
			"nodeAddress":   nodeAddressFlag,
			"clientAddress": clientAddressFlag,
		}

		if err := makeLeaderRequest("POST", "/cluster/add-node", queryParams, logger); err != nil {
			logger.Fatalln("failed to add node:", err)
		}
	},
}

var promoteNodeCmd = &cobra.Command{
	Use:   "promote-node",
	Short: "Promote a non-voter node to voter",
	Long:  `Promote a non-voter node to a voter in the Raft cluster. This command forwards the request to the leader.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

		if promoteNodeIdFlag == 0 {
			logger.Fatalln("--node-id is required")
		}

		confirmPrompt := promptui.Prompt{
			Label:       fmt.Sprintf("WARNING: This will promote node %d from non-voter to voter in the Raft cluster. Are you sure you want to continue? [y/N]:", promoteNodeIdFlag),
			HideEntered: false,
			Default:     "N",
		}
		confirm, err := confirmPrompt.Run()
		if err != nil {
			logger.Fatalln("Failed to read confirmation:", err)
		}

		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "y" && confirm != "yes" {
			logger.Println("Operation cancelled.")
			return
		}

		queryParams := map[string]string{
			"nodeId": fmt.Sprintf("%d", promoteNodeIdFlag),
		}

		if err := makeLeaderRequest("POST", "/cluster/promote-node", queryParams, logger); err != nil {
			logger.Fatalln("failed to promote node:", err)
		}
	},
}

var demoteNodeCmd = &cobra.Command{
	Use:   "demote-node",
	Short: "Demote a voter node to non-voter",
	Long:  `Demote a voter node to a non-voter in the Raft cluster. This command forwards the request to the leader.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

		if demoteNodeIdFlag == 0 {
			logger.Fatalln("--node-id is required")
		}

		confirmPrompt := promptui.Prompt{
			Label:       fmt.Sprintf("WARNING: This will demote node %d from voter to non-voter in the Raft cluster. This may affect cluster availability. Are you sure you want to continue? [y/N]:", demoteNodeIdFlag),
			HideEntered: false,
			Default:     "N",
		}
		confirm, err := confirmPrompt.Run()
		if err != nil {
			logger.Fatalln("Failed to read confirmation:", err)
		}

		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "y" && confirm != "yes" {
			logger.Println("Operation cancelled.")
			return
		}

		queryParams := map[string]string{
			"nodeId": fmt.Sprintf("%d", demoteNodeIdFlag),
		}

		if err := makeLeaderRequest("POST", "/cluster/demote-node", queryParams, logger); err != nil {
			logger.Fatalln("failed to demote node:", err)
		}
	},
}

var transferLeadershipCmd = &cobra.Command{
	Use:   "transfer-leadership",
	Short: "Transfer leadership to another node",
	Long:  `Transfer Raft leadership to another node. This command forwards the request to the current leader.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

		confirmPrompt := promptui.Prompt{
			Label:       fmt.Sprintf("WARNING: This will transfer Raft leadership to node %d. This may cause a brief leadership transition. Are you sure you want to continue? [y/N]:", targetNodeIdFlag),
			HideEntered: false,
			Default:     "N",
		}
		confirm, err := confirmPrompt.Run()
		if err != nil {
			logger.Fatalln("Failed to read confirmation:", err)
		}

		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "y" && confirm != "yes" {
			logger.Println("Operation cancelled.")
			return
		}

		if err := makeLeaderRequest("POST", "/cluster/transfer-leadership", nil, logger); err != nil {
			logger.Fatalln("failed to transfer leadership:", err)
		}
	},
}

var forceRebuildCmd = &cobra.Command{
	Use:   "force-rebuild",
	Short: "Force rebuild of the Raft cluster",
	Long:  `Force rebuild of the Raft cluster. This should only be called on the seed node. This command forwards the request to the base URL.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

		if seedNodeIdFlag == 0 {
			logger.Fatalln("--seed-node-id is required")
		}

		confirmPrompt := promptui.Prompt{
			Label:       fmt.Sprintf("WARNING: This will FORCE REBUILD the Raft cluster on seed node %d. This is a DESTRUCTIVE operation that will wipe Raft state and recreate the cluster. Are you absolutely sure you want to continue? [y/N]:", seedNodeIdFlag),
			HideEntered: false,
			Default:     "N",
		}
		confirm, err := confirmPrompt.Run()
		if err != nil {
			logger.Fatalln("Failed to read confirmation:", err)
		}

		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "y" && confirm != "yes" {
			logger.Println("Operation cancelled.")
			return
		}

		queryParams := map[string]string{
			"seedNodeId": fmt.Sprintf("%d", seedNodeIdFlag),
		}

		if err := makeLeaderRequest("POST", "/cluster/force-rebuild", queryParams, logger); err != nil {
			logger.Fatalln("failed to force rebuild cluster:", err)
		}
	},
}

var resetRaftCmd = &cobra.Command{
	Use:   "reset-raft",
	Short: "Reset local Raft state",
	Long:  `Reset local Raft state on the target node. This will clear Raft logs, stable store, and snapshots, then exit the process. This command forwards the request to the base URL.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

		confirmPrompt := promptui.Prompt{
			Label:       "WARNING: This will RESET the Raft state on the target node and cause the process to EXIT. The node will need to be restarted after this operation. Are you sure you want to continue? [y/N]:",
			HideEntered: false,
			Default:     "N",
		}
		confirm, err := confirmPrompt.Run()
		if err != nil {
			logger.Fatalln("Failed to read confirmation:", err)
		}

		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "y" && confirm != "yes" {
			logger.Println("Operation cancelled.")
			return
		}

		queryParams := map[string]string{}

		if err := makeLeaderRequest("POST", "/cluster/reset-raft", queryParams, logger); err != nil {
			logger.Fatalln("failed to reset raft state:", err)
		}
	},
}

var listNodesCmd = &cobra.Command{
	Use:   "list-nodes",
	Short: "List all nodes in the cluster",
	Long:  `List all nodes currently in the Raft cluster. This command forwards the request to the leader.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

		if err := makeLeaderRequest("GET", "/cluster/list-nodes", nil, logger); err != nil {
			logger.Fatalln("failed to list nodes:", err)
		}
	},
}

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump internal state from all cluster nodes",
	Long:  `Dump internal state (executor state and job queues) from all nodes in the cluster. This command collects data from each node and outputs it as JSON.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.New(os.Stderr, "[cmd] ", log.LstdFlags)

		// Get list of nodes
		logger.Println("Fetching list of nodes...")
		nodes, err := getNodesList(logger)
		if err != nil {
			logger.Fatalln("failed to get nodes list:", err)
		}

		if len(nodes) == 0 {
			logger.Fatalln("no nodes found in cluster")
		}

		logger.Printf("Found %d node(s), collecting dump data...\n", len(nodes))

		// Structure to hold all dump data
		type NodeDumpData struct {
			NodeId             interface{} `json:"nodeId"`
			ClientAddress      interface{} `json:"clientAddress"`
			NodeAddress        interface{} `json:"nodeAddress,omitempty"`
			IsVoter            interface{} `json:"isVoter,omitempty"`
			ScheduleQueue      interface{} `json:"scheduleQueue,omitempty"`
			JobExecutionsCache interface{} `json:"jobExecutionsCache,omitempty"`
			JobQueues          interface{} `json:"jobQueues,omitempty"`
			JobQueueVersions   interface{} `json:"jobQueueVersions,omitempty"`
			Error              string      `json:"error,omitempty"`
		}

		type DumpResult struct {
			Nodes []NodeDumpData `json:"nodes"`
		}

		result := DumpResult{
			Nodes: make([]NodeDumpData, 0, len(nodes)),
		}

		// For each node, collect dump data
		for _, node := range nodes {
			nodeDump := NodeDumpData{
				NodeId:        node["nodeId"],
				ClientAddress: node["clientAddress"],
			}

			if nodeAddress, ok := node["nodeAddress"]; ok {
				nodeDump.NodeAddress = nodeAddress
			}
			if isVoter, ok := node["isVoter"]; ok {
				nodeDump.IsVoter = isVoter
			}

			clientAddress, ok := node["clientAddress"].(string)
			if !ok {
				nodeDump.Error = "invalid clientAddress format"
				result.Nodes = append(result.Nodes, nodeDump)
				continue
			}

			logger.Printf("Collecting data from node %v (%s)...\n", node["nodeId"], clientAddress)

			// Collect schedule queue
			if body, err := makeNodeRequest(clientAddress, "GET", "/cluster/dump/schedule-queue", logger); err != nil {
				if nodeDump.Error != "" {
					nodeDump.Error += "; "
				}
				nodeDump.Error += fmt.Sprintf("failed to get schedule queue: %v", err)
			} else {
				var queue interface{}
				if err := json.Unmarshal(body, &queue); err != nil {
					if nodeDump.Error != "" {
						nodeDump.Error += "; "
					}
					nodeDump.Error += fmt.Sprintf("failed to parse schedule queue: %v", err)
				} else {
					nodeDump.ScheduleQueue = queue
				}
			}

			// Collect job executions cache
			if body, err := makeNodeRequest(clientAddress, "GET", "/cluster/dump/job-executions-cache", logger); err != nil {
				if nodeDump.Error != "" {
					nodeDump.Error += "; "
				}
				nodeDump.Error += fmt.Sprintf("failed to get job executions cache: %v", err)
			} else {
				var cache interface{}
				if err := json.Unmarshal(body, &cache); err != nil {
					if nodeDump.Error != "" {
						nodeDump.Error += "; "
					}
					nodeDump.Error += fmt.Sprintf("failed to parse job executions cache: %v", err)
				} else {
					nodeDump.JobExecutionsCache = cache
				}
			}

			// Collect job queues
			if body, err := makeNodeRequest(clientAddress, "GET", "/cluster/dump/job-queues", logger); err != nil {
				if nodeDump.Error != "" {
					nodeDump.Error += "; "
				}
				nodeDump.Error += fmt.Sprintf("failed to get job queues: %v", err)
			} else {
				var queues interface{}
				if err := json.Unmarshal(body, &queues); err != nil {
					if nodeDump.Error != "" {
						nodeDump.Error += "; "
					}
					nodeDump.Error += fmt.Sprintf("failed to parse job queues: %v", err)
				} else {
					nodeDump.JobQueues = queues
				}
			}

			// Collect job queue versions
			if body, err := makeNodeRequest(clientAddress, "GET", "/cluster/dump/job-queue-versions", logger); err != nil {
				if nodeDump.Error != "" {
					nodeDump.Error += "; "
				}
				nodeDump.Error += fmt.Sprintf("failed to get job queue versions: %v", err)
			} else {
				var versions interface{}
				if err := json.Unmarshal(body, &versions); err != nil {
					if nodeDump.Error != "" {
						nodeDump.Error += "; "
					}
					nodeDump.Error += fmt.Sprintf("failed to parse job queue versions: %v", err)
				} else {
					nodeDump.JobQueueVersions = versions
				}
			}

			result.Nodes = append(result.Nodes, nodeDump)
		}

		// Output result as JSON
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			logger.Fatalln("failed to marshal dump result:", err)
		}

		fmt.Println(string(output))
	},
}

func init() {
	removeNodeCmd.Flags().Uint64Var(&removeNodeIdFlag, "node-id", 0, "Node ID to remove")
	addNodeCmd.Flags().Uint64Var(&addNodeIdFlag, "node-id", 0, "Node ID to add")
	addNodeCmd.Flags().StringVar(&nodeAddressFlag, "node-address", "", "Node address (e.g., 127.0.0.1:8080)")
	addNodeCmd.Flags().StringVar(&clientAddressFlag, "client-address", "", "Client address (e.g., 127.0.0.1:9090)")
	promoteNodeCmd.Flags().Uint64Var(&promoteNodeIdFlag, "node-id", 0, "Node ID to promote to voter")
	demoteNodeCmd.Flags().Uint64Var(&demoteNodeIdFlag, "node-id", 0, "Node ID to demote to non-voter")
	transferLeadershipCmd.Flags().Uint64Var(&targetNodeIdFlag, "target-node-id", 0, "Target node ID for leadership transfer")
	forceRebuildCmd.Flags().Uint64Var(&seedNodeIdFlag, "seed-node-id", 0, "Seed node ID for force rebuild")
	// reset-raft and list-nodes don't need any flags

	ClusterCmd.AddCommand(removeNodeCmd)
	ClusterCmd.AddCommand(addNodeCmd)
	ClusterCmd.AddCommand(promoteNodeCmd)
	ClusterCmd.AddCommand(demoteNodeCmd)
	ClusterCmd.AddCommand(listNodesCmd)
	ClusterCmd.AddCommand(transferLeadershipCmd)
	ClusterCmd.AddCommand(forceRebuildCmd)
	ClusterCmd.AddCommand(resetRaftCmd)
	ClusterCmd.AddCommand(dumpCmd)
}
