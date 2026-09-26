package config

import (
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"scheduler0/pkg/constants"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
)

type RaftNode struct {
	ClientAddress string `json:"clientAddress" yaml:"ClientAddress"`
	NodeAddress   string `json:"nodeAddress" yaml:"NodeAddress"`
	NodeId        uint64 `json:"nodeId" yaml:"NodeId"`
}

type Scheduler0Config interface {
	GetConfigurations() *Scheduler0Configurations
}

func NewScheduler0Config() Scheduler0Config {
	return &Scheduler0Configurations{}
}

type Scheduler0Configurations struct {
	LogLevel                         string   `json:"logLevel" yaml:"LogLevel"`
	Host                             string   `json:"host" yaml:"Host"`
	ClientPort                       string   `json:"clientPort" yaml:"ClientPort"`
	NodePort                         string   `json:"nodePort" yaml:"NodePort"`
	PeerAuthRequestTimeoutMs         uint64   `json:"PeerAuthRequestTimeoutMs" yaml:"PeerAuthRequestTimeoutMs"`
	PeerConnectRetryMax              uint64   `json:"peerConnectRetryMax" yaml:"PeerConnectRetryMax"`
	PeerConnectRetryDelaySeconds     uint64   `json:"peerConnectRetryDelay" yaml:"PeerConnectRetryDelaySeconds"`
	Bootstrap                        bool     `json:"bootstrap" yaml:"Bootstrap"`
	NodeId                           uint64   `json:"nodeId" yaml:"NodeId"`
	NodeAddress                      string   `json:"nodeAddress" yaml:"NodeAddress"`
	NodeServiceDiscoveryAddress      string   `json:"nodeServiceDiscoveryAddress" yaml:"NodeServiceDiscoveryAddress"`
	RaftTransportMaxPool             uint64   `json:"raftTransportMaxPool" yaml:"RaftTransportMaxPool"`
	RaftTransportTimeout             uint64   `json:"raftTransportTimeout" yaml:"RaftTransportTimeout"`
	RaftApplyTimeout                 uint64   `json:"raftApplyTimeout" yaml:"RaftApplyTimeout"`
	RaftSnapshotInterval             uint64   `json:"raftSnapshotInterval" yaml:"RaftSnapshotInterval"`
	RaftSnapshotThreshold            uint64   `json:"raftSnapshotThreshold" yaml:"RaftSnapshotThreshold"`
	RaftHeartbeatTimeout             uint64   `json:"raftHeartbeatTimeout" yaml:"RaftHeartbeatTimeout"`
	RaftElectionTimeout              uint64   `json:"raftElectionTimeout" yaml:"RaftElectionTimeout"`
	RaftCommitTimeout                uint64   `json:"raftCommitTimeout" yaml:"RaftCommitTimeout"`
	RaftMaxAppendEntries             uint64   `json:"raftMaxAppendEntries" yaml:"RaftMaxAppendEntries"`
	JobExecutionTimeout              uint64   `json:"jobExecutionTimeout" yaml:"JobExecutionTimeout"`
	JobExecutionRetryDelay           uint64   `json:"jobExecutionRetryDelay" yaml:"JobExecutionRetryDelay"`
	MaxWorkers                       uint64   `json:"maxWorkers" yaml:"MaxWorkers"`
	MaxQueue                         uint64   `json:"maxQueue" yaml:"MaxQueue"`
	MaxMemory                        uint64   `json:"maxMemory" yaml:"MaxMemory"`
	ExecutionLogFetchFanIn           uint64   `json:"executionLogFetchFanIn" yaml:"ExecutionLogFetchFanIn"`
	ExecutionLogFetchIntervalSeconds uint64   `json:"executionLogFetchIntervalSeconds" yaml:"ExecutionLogFetchIntervalSeconds"`
	HTTPCert                         string   `json:"httpCert" yaml:"HTTPCert"`
	HTTPCertKey                      string   `json:"httpCertKey" yaml:"HTTPCertKey"`
	NodeCaCert                       string   `json:"nodeCaCert" yaml:"NodeCaCert"`
	NodeCert                         string   `json:"nodeCert" yaml:"NodeCert"`
	NodeCertKey                      string   `json:"nodeCertKey" yaml:"NodeCertKey"`
	NodeCertNoVerify                 bool     `json:"nodeCertNoVerify" yaml:"NodeCertNoVerify"`
	NodeClientCertVerify             bool     `json:"nodeClientCertVerify" yaml:"NodeClientCertVerify"`
	EtcdEndpoints                    []string `json:"etcdEndpoints" yaml:"EtcdEndpoints"`
	EtcdKeyPrefix                    string   `json:"etcdKeyPrefix" yaml:"EtcdKeyPrefix"`
	EtcdTTL                          int64    `json:"etcdTTL" yaml:"EtcdTTL"`
	ServiceDiscoveryHost             string   `json:"serviceDiscoveryHost" yaml:"ServiceDiscoveryHost"`
	S3Bucket                         string   `json:"s3Bucket" yaml:"S3Bucket"`
	AWSRegion                        string   `json:"awsRegion" yaml:"AWSRegion"`
	AIPromptProviders                string   `json:"aiPromptProviders" yaml:"AIPromptProviders"`
	AIPreferredModel                 string   `json:"aiPreferredModel" yaml:"AIPreferredModel"`
	AIBedrockModel                   string   `json:"aiBedrockModel" yaml:"AIBedrockModel"`
	OpenAIBaseURL                    string   `json:"openAiBaseUrl" yaml:"OpenAIBaseURL"`
	OpenAIAPIKey                     string   `json:"openAiApiKey" yaml:"OpenAIAPIKey"`
	OpenAIAuthToken                  string   `json:"openAiAuthToken" yaml:"OpenAIAuthToken"`
	OpenAIOrganizationID             string   `json:"openAiOrganizationId" yaml:"OpenAIOrganizationID"`
	OpenAIProjectID                  string   `json:"openAiProjectId" yaml:"OpenAIProjectID"`
	OpenRouterAPIKey                 string   `json:"openRouterApiKey" yaml:"OpenRouterAPIKey"`
	AIIntentClassifierURL            string   `json:"aiIntentClassifierUrl" yaml:"AIIntentClassifierURL"`
	PlatformAIProvider               string   `json:"platformAiProvider" yaml:"PlatformAIProvider"`
	PlatformAIModel                  string   `json:"platformAiModel" yaml:"PlatformAIModel"`
	PlatformAIMarkup                 float64  `json:"platformAiMarkup" yaml:"PlatformAIMarkup"`
	PlatformAIWelcomeCreditUSD       float64  `json:"platformAiWelcomeCreditUsd" yaml:"PlatformAIWelcomeCreditUSD"`
	PlatformWebhookURL               string   `json:"platformWebhookUrl" yaml:"PlatformWebhookURL"`
	PlatformWebhookSecret            string   `json:"platformWebhookSecret" yaml:"PlatformWebhookSecret"`
	// Env is the deployment environment name (production, staging, local).
	// Used to label operator alerts; falls back to the alerts topic ARN suffix.
	Env string `json:"env" yaml:"Env"`
	// AlertsSNSTopicARN is the per-environment ops alert topic
	// ({env}-scheduler0-alerts). Empty disables SNS alerts (they are logged).
	AlertsSNSTopicARN string `json:"alertsSnsTopicArn" yaml:"AlertsSNSTopicARN"`
}

var cachedConfig *Scheduler0Configurations

func (_ Scheduler0Configurations) GetConfigurations() *Scheduler0Configurations {
	binPath := getBinPath()

	fs := afero.NewOsFs()
	data, err := afero.ReadFile(fs, binPath+"/"+constants.ConfigFileName)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	config := Scheduler0Configurations{}

	if os.IsNotExist(err) {
		config = *getConfigFromEnv()
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		panic(err)
	}

	applyAIEnvOverrides(&config)
	applyPlatformWebhookEnvOverrides(&config)
	applyOpsAlertsEnvOverrides(&config)

	cachedConfig = &config

	return cachedConfig
}

func getConfigFromEnv() *Scheduler0Configurations {
	config := &Scheduler0Configurations{}

	if val, ok := os.LookupEnv("SCHEDULER0_LOGLEVEL"); ok {
		config.LogLevel = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_NODE_PORT"); ok {
		config.NodePort = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_CLIENT_PORT"); ok {
		config.ClientPort = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_HOST"); ok {
		config.Host = val
	}

	env := os.Getenv("ENV")
	if env == "" {
		env = os.Getenv("ENVIRONMENT")
	}
	if env == "" {
		env = os.Getenv("DEPLOY_ENV")
	}

	if (config.Host == "" || config.Host == "0.0.0.0") && env == "production" {
		if ip, err := firstPrivateIP(); err == nil && ip != "" {
			config.Host = ip
		}
	}

	if val, ok := os.LookupEnv("SCHEDULER0_PEER_AUTH_REQUEST_TIMEOUT_MS"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_PEER_AUTH_REQUEST_TIMEOUT_MS: %v", err)
		}
		config.PeerAuthRequestTimeoutMs = parsed
	}

	if val, ok := os.LookupEnv("SCHEDULER0_PEER_CONNECT_RETRY_MAX"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_PEER_CONNECT_RETRY_MAX: %v", err)
		}
		config.PeerConnectRetryMax = parsed
	}

	if val, ok := os.LookupEnv("SCHEDULER0_PEER_CONNECT_RETRY_DELAY_SECONDS"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_PEER_CONNECT_RETRY_DELAY_SECONDS: %v", err)
		}
		config.PeerConnectRetryDelaySeconds = parsed
	}

	if val, ok := os.LookupEnv("SCHEDULER0_BOOTSTRAP"); ok {
		parsed, err := strconv.ParseBool(val)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_BOOTSTRAP: %v", err)
		}
		config.Bootstrap = parsed
	}

	if val, ok := os.LookupEnv("SCHEDULER0_NODE_ID"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_NODE_ID: %v", err)
		}
		config.NodeId = parsed
	}

	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_TRANSPORT_MAX_POOL"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT")
		}
		config.RaftTransportMaxPool = parsed
	} else {
		config.RaftTransportMaxPool = 50
	}

	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_TRANSPORT_TIMEOUT"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_TRANSPORT_TIMEOUT")
		}
		config.RaftTransportTimeout = parsed
	} else {
		config.RaftTransportTimeout = 120
	}

	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_SNAPSHOT_INTERVAL"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_SNAPSHOT_INTERVAL: %v", err)
		}
		config.RaftSnapshotInterval = parsed
	}

	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_SNAPSHOT_THRESHOLD"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_SNAPSHOT_THRESHOLD: %v", err)
		}
		config.RaftSnapshotThreshold = parsed
	}

	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_HEARTBEAT_TIMEOUT"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_HEARTBEAT_TIMEOUT: %v", err)
		}
		config.RaftHeartbeatTimeout = parsed
	} else {
		config.RaftHeartbeatTimeout = 2000
	}

	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_ELECTION_TIMEOUT"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_ELECTION_TIMEOUT: %v", err)
		}
		config.RaftElectionTimeout = parsed
	} else {
		config.RaftElectionTimeout = 2000
	}
	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_COMMIT_TIMEOUT"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_COMMIT_TIMEOUT: %v", err)
		}
		config.RaftCommitTimeout = parsed
	}
	if val, ok := os.LookupEnv("SCHEDULER0_RAFT_MAX_APPEND_ENTRIES"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_RAFT_MAX_APPEND_ENTRIES: %v", err)
		}
		config.RaftMaxAppendEntries = parsed
	}

	if val, ok := os.LookupEnv("SCHEDULER0_JOB_EXECUTION_TIMEOUT"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_JOB_EXECUTION_TIMEOUT: %v", err)
		}
		config.JobExecutionTimeout = parsed
	} else {
		// Seconds. Every consumer multiplies this by time.Second (webhook
		// executor HTTP client timeout, test-invocation wait in executor.go), and
		// config.yml / readme document it in seconds. The previous default of
		// 30000 was a millisecond-scale value in a seconds field and produced an
		// 8.3-hour HTTP client timeout.
		config.JobExecutionTimeout = 30
	}

	if val, ok := os.LookupEnv("SCHEDULER0_JOB_EXECUTION_RETRY_DELAY"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_JOB_EXECUTION_RETRY_DELAY: %v", err)
		}
		config.JobExecutionRetryDelay = parsed
	} else {
		// Seconds, consumed by utils.RetryOnError (time.Second * delay). The
		// previous default of 5000 meant a failing webhook slept 83 minutes
		// between attempts and took 3 x 5000 s = 4h10m to be marked failed.
		config.JobExecutionRetryDelay = 5
	}

	if val, ok := os.LookupEnv("SCHEDULER0_MAX_WORKERS"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_MAX_WORKERS: %v", err)
		}
		config.MaxWorkers = parsed
	} else {
		config.MaxWorkers = 10
	}

	if val, ok := os.LookupEnv("SCHEDULER0_MAX_QUEUE"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_MAX_QUEUE: %v", err)
		}
		config.MaxQueue = parsed
	} else {
		config.MaxQueue = 1
	}

	if val, ok := os.LookupEnv("SCHEDULER0_MAX_MEMORY"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_MAX_MEMORY: %v", err)
		}
		config.MaxMemory = parsed
	} else {
		config.MaxMemory = 1024 * 1024 * 1024
	}

	if val, ok := os.LookupEnv("SCHEDULER0_EXECUTION_LOG_FETCH_FAN_IN"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_EXECUTION_LOG_FETCH_FAN_IN: %v", err)
		}
		config.ExecutionLogFetchFanIn = parsed
	} else {
		config.ExecutionLogFetchFanIn = 2
	}

	if val, ok := os.LookupEnv("SCHEDULER0_EXECUTION_LOG_FETCH_INTERVAL_SECONDS"); ok {
		parsed, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_EXECUTION_LOG_FETCH_INTERVAL_SECONDS: %v", err)
		}
		config.ExecutionLogFetchIntervalSeconds = parsed
	} else {
		config.ExecutionLogFetchIntervalSeconds = 120
	}

	if val, ok := os.LookupEnv("SCHEDULER0_HTTP_CERT"); ok {
		config.HTTPCert = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_HTTP_CERT_KEY"); ok {
		config.HTTPCertKey = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_NODE_CA_CERT"); ok {
		config.NodeCaCert = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_NODE_CERT"); ok {
		config.NodeCert = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_NODE_CERT_KEY"); ok {
		config.NodeCertKey = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_NODE_CERT_NO_VERIFY"); ok {
		parsed, err := strconv.ParseBool(val)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_NODE_CERT_NO_VERIFY: %v", err)
		}
		config.NodeCertNoVerify = parsed
	}

	if val, ok := os.LookupEnv("SCHEDULER0_NODE_CLIENT_CERT_VERIFY"); ok {
		parsed, err := strconv.ParseBool(val)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_NODE_CLIENT_CERT_VERIFY: %v", err)
		}
		config.NodeClientCertVerify = parsed
	}

	if val, ok := os.LookupEnv("SCHEDULER0_ETCD_ENDPOINTS"); ok {
		endpoints := strings.Split(val, ",")
		for i := range endpoints {
			endpoints[i] = strings.TrimSpace(endpoints[i])
		}
		config.EtcdEndpoints = endpoints
	}

	if val, ok := os.LookupEnv("SCHEDULER0_ETCD_KEY_PREFIX"); ok {
		config.EtcdKeyPrefix = val
	} else {
		config.EtcdKeyPrefix = "/scheduler0/nodes"
	}

	if val, ok := os.LookupEnv("SCHEDULER0_ETCD_TTL"); ok {
		parsed, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			log.Fatalf("Error parsing SCHEDULER0_ETCD_TTL: %v", err)
		}
		config.EtcdTTL = parsed
	} else {
		config.EtcdTTL = 30
	}

	if val, ok := os.LookupEnv("SCHEDULER0_SERVICE_DISCOVERY_HOST"); ok {
		config.ServiceDiscoveryHost = val
	}

	config.NodeAddress = fmt.Sprintf("%s:%s", config.Host, config.NodePort)

	config.NodeServiceDiscoveryAddress = fmt.Sprintf("%s:%s", config.ServiceDiscoveryHost, config.NodePort)

	if val, ok := os.LookupEnv("SCHEDULER0_S3_BUCKET"); ok {
		config.S3Bucket = val
	}

	if val, ok := os.LookupEnv("SCHEDULER0_AWS_REGION"); ok {
		config.AWSRegion = val
	}

	applyAIEnvOverrides(config)
	applyPlatformWebhookEnvOverrides(config)
	applyOpsAlertsEnvOverrides(config)

	return config
}

func applyAIEnvOverrides(config *Scheduler0Configurations) {
	if val, ok := os.LookupEnv("SCHEDULER0_AI_PROMPT_PROVIDERS"); ok {
		config.AIPromptProviders = val
	}
	if val, ok := os.LookupEnv("SCHEDULER0_AI_MODEL"); ok {
		config.AIPreferredModel = val
	}
	if val, ok := os.LookupEnv("SCHEDULER0_AI_BEDROCK_MODEL"); ok {
		config.AIBedrockModel = val
	}
	if val, ok := os.LookupEnv("SCHEDULER0_OPENAI_BASE_URL"); ok {
		config.OpenAIBaseURL = val
	}
	if val, ok := os.LookupEnv("SCHEDULER0_OPENAI_API_KEY"); ok {
		config.OpenAIAPIKey = val
	}
	if val, ok := os.LookupEnv("SCHEDULER0_OPENAI_AUTH_TOKEN"); ok {
		config.OpenAIAuthToken = val
	}
	if val, ok := os.LookupEnv("SCHEDULER0_OPENAI_ORGANIZATION_ID"); ok {
		config.OpenAIOrganizationID = val
	}
	if val, ok := os.LookupEnv("SCHEDULER0_OPENAI_PROJECT_ID"); ok {
		config.OpenAIProjectID = val
	}
	if val, ok := os.LookupEnv("SCHEDULER0_OPENROUTER_API_KEY"); ok {
		config.OpenRouterAPIKey = val
	}
	if val, ok := os.LookupEnv("SCHEDULER0_AI_INTENT_CLASSIFIER_URL"); ok {
		config.AIIntentClassifierURL = val
	} else {
		config.AIIntentClassifierURL = "http://127.0.0.1:5001"
	}
	applyPlatformAIEnvOverrides(config)
}

func applyPlatformAIEnvOverrides(config *Scheduler0Configurations) {
	if val, ok := os.LookupEnv("SCHEDULER0_PLATFORM_AI_PROVIDER"); ok {
		config.PlatformAIProvider = val
	}
	if config.PlatformAIProvider == "" {
		config.PlatformAIProvider = "bedrock"
	}
	if val, ok := os.LookupEnv("SCHEDULER0_PLATFORM_AI_MODEL"); ok {
		config.PlatformAIModel = val
	}
	if val, ok := os.LookupEnv("SCHEDULER0_PLATFORM_AI_MARKUP"); ok {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			config.PlatformAIMarkup = parsed
		}
	}
	if config.PlatformAIMarkup <= 0 {
		config.PlatformAIMarkup = 1.20
	}
	if val, ok := os.LookupEnv("SCHEDULER0_PLATFORM_AI_WELCOME_CREDIT_USD"); ok {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			config.PlatformAIWelcomeCreditUSD = parsed
		}
	}
	if config.PlatformAIWelcomeCreditUSD <= 0 {
		config.PlatformAIWelcomeCreditUSD = 5.0
	}
}

func applyPlatformWebhookEnvOverrides(config *Scheduler0Configurations) {
	if val, ok := os.LookupEnv("PLATFORM_WEBHOOK_URL"); ok {
		config.PlatformWebhookURL = val
	}
	if val, ok := os.LookupEnv("PLATFORM_WEBHOOK_SECRET"); ok {
		config.PlatformWebhookSecret = val
	}
}

// applyOpsAlertsEnvOverrides wires the operator-alert SNS publisher. Both are
// plain env vars set by the deploy workflow (non-secret), so they are honoured
// whether the node booted from config.yml or purely from env.
func applyOpsAlertsEnvOverrides(config *Scheduler0Configurations) {
	if val, ok := os.LookupEnv("ALERTS_SNS_TOPIC_ARN"); ok {
		config.AlertsSNSTopicARN = val
	}
	if val, ok := os.LookupEnv("SCHEDULER0_ENV"); ok {
		config.Env = val
	}
}

func firstPrivateIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			if ip[0] == 10 || (ip[0] == 172 && ip[1]&0xf0 == 16) || (ip[0] == 192 && ip[1] == 168) {
				return ip.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no private IP found")
}

func getBinPath() string {
	e, err := os.Executable()
	if err != nil {
		log.Fatalln("failed to get path of scheduler0 binary", err.Error())
	}
	return path.Dir(e)
}
