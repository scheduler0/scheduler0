# Getting Started with Scheduler0

This guide will help you get Scheduler0 up and running in just a few minutes.

## Prerequisites

Before you begin, ensure you have:

- **Go 1.26.5 or later** (if building from source)
- **etcd** (required for distributed operation)
- **gcc** (required for CGO/SQLite)

## Installation

### Option 1: Build from Source

```bash
# Clone the repository
git clone https://github.com/scheduler0/scheduler0.git
cd scheduler0

# Build the binary (CGO is required for SQLite)
CGO_ENABLED=1 go build -o scheduler0 ./

# Verify the build
./scheduler0 version
```

### Option 2: Download Pre-built Binary

```bash
# Download the latest release (replace with actual version)
curl -LO https://github.com/scheduler0/scheduler0/releases/latest/download/scheduler0-linux-amd64

# Make it executable
chmod +x scheduler0-linux-amd64
mv scheduler0-linux-amd64 scheduler0

# Verify
./scheduler0 version
```

### Option 3: Using Docker

```bash
# Pull the image
docker pull scheduler0/scheduler0:latest

# Verify
docker run scheduler0/scheduler0:latest version
```

## Setup

### Step 1: Install and Start etcd

Scheduler0 requires etcd for service discovery. Install and start it:

```bash
# Download etcd (v3.5.x or later)
ETCD_VER=v3.5.9
curl -L https://github.com/etcd-io/etcd/releases/download/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz -o etcd.tar.gz
tar xzvf etcd.tar.gz
sudo mv etcd-${ETCD_VER}-linux-amd64/etcd* /usr/local/bin/

# Start etcd
etcd --data-dir /tmp/etcd-data.etcd \
     --listen-client-urls http://0.0.0.0:2379 \
     --advertise-client-urls http://localhost:2379 &
```

### Step 2: Initialize the Database

```bash
# Create a directory for your node
mkdir -p /tmp/scheduler0-node1
cd /tmp/scheduler0-node1

# Copy the scheduler0 binary here
cp /path/to/scheduler0 .

# Initialize the database (creates SQLite DB and runs migrations)
./scheduler0 init
```

This creates:
- `sqlite_data/scheduler0.db` - The main database
- Seeds a "System" account with ID 1

### Step 3: Configure Secrets

Scheduler0 needs secrets for authentication. Set them via environment variables:

```bash
# Generate a secret key (must be 32, 48, or 64 hex characters)
export SCHEDULER0_SECRET_KEY=$(openssl rand -hex 32)

# Set authentication credentials
export SCHEDULER0_AUTH_USERNAME="admin"
export SCHEDULER0_AUTH_PASSWORD="your-secure-password"
```

**Alternative**: Use the interactive secrets setup:

```bash
./scheduler0 secrets init
```

This creates a `.scheduler0` file with your secrets.

### Step 4: Configure the Node

Set the required environment variables for a single-node bootstrap:

```bash
# Node identity
export SCHEDULER0_NODE_ID=1
export SCHEDULER0_HOST=127.0.0.1
export SCHEDULER0_CLIENT_PORT=9090  # Client API port
export SCHEDULER0_NODE_PORT=8080    # Inter-node communication port

# Service discovery
export SCHEDULER0_SERVICE_DISCOVERY_HOST=127.0.0.1
export SCHEDULER0_ETCD_ENDPOINTS=http://localhost:2379

# Bootstrap mode (only first node should be true)
export SCHEDULER0_BOOTSTRAP=true
```

### Step 5: Start the Node

```bash
./scheduler0 start
```

You should see output like:
```
[INFO] Starting Scheduler0 node...
[INFO] Node ID: 1
[INFO] Client API: http://127.0.0.1:9090
[INFO] Node Port: 8080
[INFO] etcd endpoints: http://localhost:2379
[INFO] Entering leader state
[INFO] HTTP server listening on :9090
```

The node is now running!

## Creating Your First Job

### Step 1: Create API Credentials

```bash
./scheduler0 create credential --created-by admin
```

This outputs:
```json
{
  "apiKey": "sk_...",
  "secretKey": "ssk_...",
  "accountId": 1
}
```

**Save these credentials** - you'll need them for API requests.

### Step 2: Create a Project

```bash
curl -X POST http://localhost:9090/api/v1/projects \
  -H "X-API-Key: sk_..." \
  -H "X-Secret-Key: ssk_..." \
  -H "X-Account-ID: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My First Project",
    "description": "Getting started with Scheduler0"
  }'
```

Response:
```json
{
  "success": true,
  "data": {
    "id": 1,
    "name": "My First Project",
    "description": "Getting started with Scheduler0",
    "accountId": 1,
    "dateCreated": "2026-09-26T05:20:00Z"
  }
}
```

### Step 3: Create a Webhook Executor

```bash
curl -X POST http://localhost:9090/api/v1/executors \
  -H "X-API-Key: sk_..." \
  -H "X-Secret-Key: ssk_..." \
  -H "X-Account-ID: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "executorType": "webhook_url",
    "webhookUrl": "https://webhook.site/your-unique-id"
  }'
```

**Tip**: Get a test webhook URL at https://webhook.site

### Step 4: Schedule a Job

```bash
curl -X POST http://localhost:9090/api/v1/jobs \
  -H "X-API-Key: sk_..." \
  -H "X-Secret-Key: ssk_..." \
  -H "X-Account-ID: 1" \
  -H "Content-Type: application/json" \
  -d '[{
    "projectId": 1,
    "executorId": 1,
    "spec": "@every 1m",
    "data": "{\"message\": \"Hello from Scheduler0!\"}",
    "createdBy": "admin",
    "timezone": "America/New_York"
  }]'
```

**Cron Spec Formats**:
- `@every 1m` - Every minute
- `@every 5m` - Every 5 minutes
- `0 */6 * * *` - Every 6 hours
- `0 9 * * 1-5` - Weekdays at 9 AM

### Step 5: Verify Execution

Check your webhook.site URL - you should see POST requests arriving every minute!

Or check execution logs:
```bash
curl http://localhost:9090/api/v1/jobs/1/executions \
  -H "X-API-Key: sk_..." \
  -H "X-Secret-Key: ssk_..." \
  -H "X-Account-ID: 1"
```

## Next Steps

Now that you have Scheduler0 running:

- **[Explore the API](API-Reference)** - Learn all API endpoints
- **[Try AI Scheduling](AI-Scheduling)** - Use natural language to create schedules
- **[Run a Cluster](Running-a-Cluster)** - Scale to multiple nodes
- **[Configure Executors](Executors)** - Use Lambda, Azure, or GCP
- **[Production Deployment](Production-Best-Practices)** - Deploy to production

## Troubleshooting

### Node won't start

**Error**: `failed to connect to etcd`
- **Solution**: Ensure etcd is running on localhost:2379

**Error**: `empty address in configuration`
- **Solution**: Remove any `config.yml` file. Use environment variables only.

**Error**: `panic: invalid secret key length`
- **Solution**: Secret key must be 32, 48, or 64 hex characters (16, 24, or 32 bytes)

### Jobs not executing

1. Check the job was created: `GET /api/v1/jobs`
2. Check execution logs: `GET /api/v1/jobs/:id/executions`
3. Verify executor webhook URL is reachable
4. Check node logs for errors

See the [Troubleshooting Guide](Troubleshooting) for more help.

## Clean Up

To stop and clean up:

```bash
# Stop Scheduler0 (Ctrl+C)

# Stop etcd
pkill etcd

# Remove data (optional)
rm -rf /tmp/scheduler0-node1
rm -rf /tmp/etcd-data.etcd
```

---

**Next**: [Running a Cluster](Running-a-Cluster) | [API Reference](API-Reference)
