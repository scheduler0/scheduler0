# Architecture Overview

Scheduler0 is designed as a distributed, cloud-native cron-job scheduler built on proven distributed systems principles.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Layer                          │
│  (REST API, CLI, SDKs)                                      │
└───────────────────────┬─────────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────────┐
│                   API Gateway / Load Balancer                │
└───────────────────────┬─────────────────────────────────────┘
                        │
        ┌───────────────┴───────────────┐
        │                               │
┌───────▼────────┐            ┌────────▼────────┐
│  Scheduler0    │◄─────Raft──┤  Scheduler0     │
│  Node 1        │   Consensus│  Node 2         │
│  (Leader)      │            │  (Follower)     │
└───┬────────┬───┘            └────┬────────┬───┘
    │        │                     │        │
    │   ┌────▼─────────────────────▼───┐    │
    │   │      etcd Cluster             │    │
    │   │  (Service Discovery)          │    │
    │   └───────────────────────────────┘    │
    │                                        │
    │   ┌────────────────────────────────┐  │
    └───►   Job Executors                │◄─┘
        │   - Webhooks                   │
        │   - AWS Lambda                 │
        │   - Azure Functions            │
        │   - GCP Cloud Functions        │
        └────────────────────────────────┘
```

## Core Components

### 1. Raft Consensus Layer

**Purpose**: Ensures data consistency and leader election

**Key Features**:
- **Leader Election**: Automatic leader election when nodes fail
- **Log Replication**: All writes go through the leader and replicate to followers
- **State Machine**: Job schedules and configurations stored in Raft log
- **Snapshots**: Periodic snapshots prevent log from growing indefinitely

**Ports**:
- Node Port (default 8080): Raft inter-node communication

### 2. Service Discovery (etcd)

**Purpose**: Dynamic peer discovery and health monitoring

**Key Features**:
- **Peer Registration**: Nodes register themselves on startup
- **Health Checks**: Automatic detection of failed nodes
- **Dynamic Membership**: Add/remove nodes without downtime
- **Leader Discovery**: Clients find the current leader

**Why etcd**:
- Battle-tested in Kubernetes
- Built-in health checking
- Efficient watch mechanism
- High availability

### 3. Storage Layer

**SQLite (Primary Data)**:
- Job definitions
- Execution logs
- Projects and credentials
- Account information

**BoltDB (Raft State)**:
- Raft logs
- Raft snapshots
- Cluster membership

**Why Embedded Storage**:
- Simple deployment (no external DB)
- Strong consistency via Raft
- Local disk performance
- Easy backups

### 4. HTTP API Server

**Purpose**: REST API for job management

**Ports**:
- Client Port (default 9090): HTTP API

**Endpoints**:
- `/api/v1/jobs` - Job CRUD
- `/api/v1/projects` - Project management
- `/api/v1/executors` - Executor configuration
- `/api/v1/cluster` - Cluster operations

**Authentication**:
- Node-to-node: Basic Auth
- Client API: API Key + Secret Key
- Peer operations: X-Peer header

### 5. Job Scheduler

**Components**:
- **Processor**: Parses cron specs and enqueues jobs
- **Queue Service**: Distributes jobs across nodes
- **Executor Service**: Dispatches jobs to executors

**Scheduling Flow**:
```
1. Processor reads jobs from DB
2. Evaluates cron expression
3. If job should run now:
   - Enqueue to job queue
   - Executor picks from queue
   - Invokes configured executor
   - Records execution log
```

**Load Balancing**:
- Jobs distributed across non-leader nodes
- Leader only executes jobs if it's the only node
- Round-robin assignment by default

### 6. Executors

**Webhook Executor**:
- HTTP POST to configured URL
- Sends job data + metadata
- Configurable retry and timeout

**Cloud Function Executors**:
- **AWS Lambda**: Direct invocation via AWS SDK
- **Azure Functions**: HTTP trigger
- **GCP Cloud Functions**: HTTP trigger

**Executor Selection**:
- Per-job executor configuration
- Account-level defaults
- Failover and retry logic

## Data Flow

### Job Creation Flow

```
Client
  │
  ├─► POST /api/v1/jobs
  │
  ▼
API Gateway (Any Node)
  │
  ├─► Forward to Leader
  │
  ▼
Leader Node
  │
  ├─► Validate & Authorize
  ├─► Write to Raft Log
  ├─► Replicate to Followers
  │
  ▼
Followers
  │
  ├─► Apply to SQLite
  │
  ▼
Response to Client
```

### Job Execution Flow

```
Processor (Leader)
  │
  ├─► Read jobs from DB
  ├─► Evaluate cron specs
  ├─► Filter due jobs
  │
  ▼
Job Queue Service
  │
  ├─► Distribute to nodes
  │
  ▼
Executor Service (Follower)
  │
  ├─► Pick job from queue
  ├─► Invoke executor (webhook/lambda/etc)
  ├─► Wait for response
  ├─► Retry on failure
  │
  ▼
Record Execution Log
  │
  ├─► Write to leader
  ├─► Replicate via Raft
```

## Failure Scenarios

### Leader Failure

1. Leader stops responding
2. Followers detect missing heartbeats
3. New leader election initiated
4. New leader elected within seconds
5. Job scheduling resumes
6. In-flight jobs may execute twice (at-least-once semantics)

### Follower Failure

1. Follower stops responding
2. etcd marks node as unhealthy
3. Leader stops assigning jobs to failed node
4. Jobs redistributed to healthy nodes
5. No data loss (data on leader)

### Network Partition

**Split Brain Prevention**:
- Raft requires majority quorum
- Minority partition cannot make progress
- Only majority partition serves requests

**Recommended**:
- Deploy odd number of nodes (3, 5, 7)
- Use anti-affinity rules (different AZs)

### etcd Failure

**Impact**:
- New nodes cannot join
- Leader discovery delayed
- Existing cluster continues to function

**Mitigation**:
- Run etcd as a cluster (3+ nodes)
- Configure multiple etcd endpoints

## Consistency Guarantees

- **Strong Consistency**: All reads from leader see latest writes
- **Linearizability**: Operations appear to execute atomically
- **At-Least-Once Execution**: Jobs may execute multiple times during failures
- **Ordered Execution**: Jobs from same project execute in schedule order

## Scalability

**Vertical Scaling**:
- More CPU: Handle more concurrent job executions
- More Memory: Cache more job schedules
- Faster Disk: Improve Raft log performance

**Horizontal Scaling**:
- Add follower nodes for more execution capacity
- Recommended: 3-7 nodes for most workloads
- Leader is bottleneck for writes (10k+ jobs/sec)

**Performance Characteristics**:
- Job creation: ~1000/sec (leader throughput)
- Job execution: ~10k/sec (distributed across followers)
- Cluster size: 3-7 nodes recommended
- Job count: Tested with 100k+ active jobs

## Security Model

**Authentication Layers**:
1. **Node-to-Node**: Basic Auth (shared username/password)
2. **Client API**: API Key + Secret Key (AES encrypted)
3. **Peer Operations**: X-Peer header validation

**Secret Storage**:
- Secrets encrypted at rest (AES-256)
- Multi-source secrets (file, AWS, env)
- Rotation support via secret_rotation service

**Network Security**:
- TLS support for client and node ports
- mTLS for inter-node communication (future)

## Monitoring Points

**Health Checks**:
- `GET /api/v1/healthcheck` - Node health
- `GET /api/v1/cluster/peers` - Cluster status

**Key Metrics**:
- Raft leader elections
- Job execution success/failure rate
- Queue depth per node
- etcd connectivity
- API latency

**Logging**:
- Structured JSON logs
- Request ID tracing
- Execution logs per job

---

**Next**: [Configuration Guide](Configuration) | [Running a Cluster](Running-a-Cluster)
