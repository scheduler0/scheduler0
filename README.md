<p align="center">
    <img src="./logo.png" height="250" />
    <br /><br />
    <a href='https://coveralls.io/github/scheduler0/scheduler0?branch=main'><img src='https://coveralls.io/repos/github/scheduler0/scheduler0/badge.svg?branch=main&service=github' alt='Coverage Status' /></a>
    <a href='https://github.com/scheduler0/scheduler0/blob/main/LICENSE'><img src='https://img.shields.io/badge/License-MIT-blue.svg' alt='License: MIT' /></a>
</p>

# Scheduler0

A cloud-native distributed cron-job scheduler built on Raft consensus, etcd service discovery, and embedded SQLite storage. Schedule jobs across multiple nodes with automatic failover, natural language scheduling via AI, and multi-cloud executor support.

## ✨ Features

- **🌐 Distributed Architecture**: Raft-based consensus with automatic leader election and state replication
- **📡 Service Discovery**: etcd integration for dynamic peer discovery
- **🤖 AI-Powered Scheduling**: Natural language to cron conversion using OpenAI, Claude, Bedrock, or OpenRouter
- **☁️ Multi-Cloud Executors**: 
  - HTTP Webhooks
  - AWS Lambda
  - Azure Functions  
  - Google Cloud Functions
- **🔒 Multi-Source Secrets**: File, AWS SSM, AWS Secrets Manager, or environment variables
- **📊 Execution Tracking**: Comprehensive job execution logs and history
- **🚀 High Availability**: Job execution continues during leadership transitions
- **⚡ Job Recovery**: Recovers missed jobs after node restarts
- **🎯 Load Balancing**: Automatic job distribution across cluster nodes
- **📦 Easy Deployment**: Single binary with embedded database

## 🚀 Quick Start

### Prerequisites

- Go 1.26.5 or later (for building from source)
- etcd (required for distributed operation)

### Installation

**Build from source:**
```bash
CGO_ENABLED=1 go build -o scheduler0 ./
```

**Using Docker:**
```bash
docker pull scheduler0/scheduler0:latest
```

### Running a Single Node

1. **Initialize the database:**
```bash
./scheduler0 init
```

2. **Set up secrets:**
```bash
export SCHEDULER0_SECRET_KEY="your-32-48-or-64-hex-char-key"
export SCHEDULER0_AUTH_USERNAME="admin"
export SCHEDULER0_AUTH_PASSWORD="secure-password"
```

3. **Start etcd:**
```bash
etcd --data-dir /tmp/etcd-data.etcd \
     --listen-client-urls http://0.0.0.0:2379 \
     --advertise-client-urls http://localhost:2379
```

4. **Start Scheduler0:**
```bash
export SCHEDULER0_HOST=127.0.0.1
export SCHEDULER0_CLIENT_PORT=9090
export SCHEDULER0_NODE_PORT=8080
export SCHEDULER0_SERVICE_DISCOVERY_HOST=127.0.0.1
export SCHEDULER0_BOOTSTRAP=true
export SCHEDULER0_NODE_ID=1
export SCHEDULER0_ETCD_ENDPOINTS=http://localhost:2379

./scheduler0 start
```

### Quick Test

Create a credential and schedule your first job:

```bash
# Create API credentials
./scheduler0 create credential --created-by admin

# Use the returned API key and secret to create a job
curl -X POST http://localhost:9090/api/v1/projects \
  -H "X-API-Key: your-api-key" \
  -H "X-Secret-Key: your-secret-key" \
  -H "X-Account-ID: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My First Project",
    "description": "Test project"
  }'

# Create a webhook executor
curl -X POST http://localhost:9090/api/v1/executors \
  -H "X-API-Key: your-api-key" \
  -H "X-Secret-Key: your-secret-key" \
  -H "X-Account-ID: 1" \
  -d '{
    "executorType": "webhook_url",
    "webhookUrl": "https://your-webhook-endpoint.com/callback"
  }'

# Schedule a job
curl -X POST http://localhost:9090/api/v1/jobs \
  -H "X-API-Key: your-api-key" \
  -H "X-Secret-Key: your-secret-key" \
  -H "X-Account-ID: 1" \
  -d '[{
    "projectId": 1,
    "executorId": 1,
    "spec": "@every 5m",
    "data": "{}",
    "createdBy": "admin"
  }]'
```

## 📖 Documentation

- **[Getting Started Guide](https://github.com/scheduler0/scheduler0/wiki/Getting-Started)** - Complete setup walkthrough
- **[Architecture Overview](https://github.com/scheduler0/scheduler0/wiki/Architecture)** - How Scheduler0 works
- **[API Reference](https://github.com/scheduler0/scheduler0/wiki/API-Reference)** - Complete REST API documentation
- **[Configuration Guide](https://github.com/scheduler0/scheduler0/wiki/Configuration)** - All configuration options
- **[Running a Cluster](https://github.com/scheduler0/scheduler0/wiki/Running-a-Cluster)** - Multi-node deployment
- **[AI Scheduling](https://github.com/scheduler0/scheduler0/wiki/AI-Scheduling)** - Natural language job scheduling
- **[Executors Guide](https://github.com/scheduler0/scheduler0/wiki/Executors)** - Using different execution backends
- **[Troubleshooting](https://github.com/scheduler0/scheduler0/wiki/Troubleshooting)** - Common issues and solutions

## 🏗️ Architecture

Scheduler0 uses a distributed architecture with these key components:

- **Raft Consensus**: Ensures data consistency across nodes with leader election
- **etcd Service Discovery**: Dynamic peer discovery and health monitoring
- **SQLite + BoltDB**: Embedded storage for jobs and Raft state
- **Dual-Port Design**: Separate client (9090) and node (8080) ports
- **Job Queue**: Distributed job queue with account-level quotas

See the [Architecture Wiki](https://github.com/scheduler0/scheduler0/wiki/Architecture) for details.

## 🔧 Configuration

Scheduler0 can be configured via:
- **Environment variables** (recommended for production)
- **config.yml file** (for development)

### Key Environment Variables

```bash
# Node Identity
SCHEDULER0_NODE_ID=1
SCHEDULER0_HOST=127.0.0.1
SCHEDULER0_CLIENT_PORT=9090
SCHEDULER0_NODE_PORT=8080

# Bootstrap (only one node should have this true)
SCHEDULER0_BOOTSTRAP=true

# etcd Service Discovery
SCHEDULER0_ETCD_ENDPOINTS=http://localhost:2379
SCHEDULER0_SERVICE_DISCOVERY_HOST=127.0.0.1

# Secrets (required)
SCHEDULER0_SECRET_KEY=your-hex-key
SCHEDULER0_AUTH_USERNAME=admin
SCHEDULER0_AUTH_PASSWORD=password
```

See the [Configuration Wiki](https://github.com/scheduler0/scheduler0/wiki/Configuration) for all options.

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Setup

1. Clone the repository
2. Install Go 1.26.5+
3. Install etcd
4. Run tests: `CGO_ENABLED=1 go test ./...`
5. Build: `CGO_ENABLED=1 go build -o scheduler0 ./`

## 📊 API Overview

Scheduler0 exposes a REST API at `/api/v1`:

### Core Resources
- `POST /api/v1/credentials` - Create API credentials
- `POST /api/v1/projects` - Create projects
- `POST /api/v1/executors` - Create executors
- `POST /api/v1/jobs` - Schedule jobs
- `GET /api/v1/jobs/:id/executions` - Get execution logs

### AI Features
- `POST /api/v1/ai/classify` - Convert natural language to cron
- `POST /api/v1/ai/suggest` - Get schedule optimization suggestions

### Cluster Management
- `GET /api/v1/cluster/peers` - List cluster nodes
- `POST /api/v1/cluster/transfer-leadership` - Transfer Raft leadership

See the [API Reference Wiki](https://github.com/scheduler0/scheduler0/wiki/API-Reference) for complete documentation.

## 🐳 Docker Deployment

### Single Node
```bash
docker run -d \
  -p 9090:9090 \
  -p 8080:8080 \
  -e SCHEDULER0_NODE_ID=1 \
  -e SCHEDULER0_HOST=0.0.0.0 \
  -e SCHEDULER0_CLIENT_PORT=9090 \
  -e SCHEDULER0_NODE_PORT=8080 \
  -e SCHEDULER0_BOOTSTRAP=true \
  -e SCHEDULER0_ETCD_ENDPOINTS=http://etcd:2379 \
  -e SCHEDULER0_SECRET_KEY=your-key \
  -e SCHEDULER0_AUTH_USERNAME=admin \
  -e SCHEDULER0_AUTH_PASSWORD=password \
  scheduler0/scheduler0:latest
```

### Cluster with Docker Compose

See the [Running a Cluster Wiki](https://github.com/scheduler0/scheduler0/wiki/Running-a-Cluster) for complete Docker Compose examples.

## 🔐 Security

- **Authentication**: Basic auth for node-to-node communication
- **API Keys**: AES-encrypted API keys and secrets for client authentication
- **Secret Rotation**: Built-in credential re-encryption support
- **Multi-Source Secrets**: AWS Secrets Manager, AWS SSM, file, or environment variables

## 📈 Monitoring & Observability

- **Health Checks**: `GET /api/v1/healthcheck`
- **Cluster Status**: `GET /api/v1/cluster/peers`
- **Execution Logs**: Comprehensive job execution history
- **SNS Alerts**: Operator alerts for critical failures (AWS)

## 🗺️ Roadmap

- [ ] Web UI for job management
- [ ] Prometheus metrics export
- [ ] PostgreSQL storage backend
- [ ] Job chaining and dependencies
- [ ] Webhook payload templates
- [ ] Rate limiting per executor
- [ ] Multi-tenancy improvements

## 🐛 Troubleshooting

### Node won't start
- Ensure etcd is running and reachable
- Check `SCHEDULER0_ETCD_ENDPOINTS` is correct
- Verify no `config.yml` file exists (use env vars instead)

### Jobs not executing
- Verify executor webhook URL is reachable
- Check execution logs: `GET /api/v1/jobs/:id/executions`
- Ensure account has not exceeded quota

See the [Troubleshooting Wiki](https://github.com/scheduler0/scheduler0/wiki/Troubleshooting) for more solutions.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Raft Consensus](https://raft.github.io/) via Hashicorp's implementation
- Service discovery powered by [etcd](https://etcd.io/)
- Job scheduling using [robfig/cron](https://github.com/robfig/cron)
- AI integrations with OpenAI, Anthropic, and AWS Bedrock

## 📞 Support

- **Documentation**: [GitHub Wiki](https://github.com/scheduler0/scheduler0/wiki)
- **Issues**: [GitHub Issues](https://github.com/scheduler0/scheduler0/issues)
- **Discussions**: [GitHub Discussions](https://github.com/scheduler0/scheduler0/discussions)

---

Made with ❤️ by the Scheduler0 team
