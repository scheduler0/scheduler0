# Troubleshooting Guide

Common issues and solutions for Scheduler0.

## Table of Contents

- [Installation & Setup](#installation--setup)
- [Node Startup Issues](#node-startup-issues)
- [Job Execution Issues](#job-execution-issues)
- [Cluster Issues](#cluster-issues)
- [Performance Issues](#performance-issues)
- [Database Issues](#database-issues)

---

## Installation & Setup

### Build Fails with "undefined: sqlite3"

**Symptoms**:
```
# scheduler0/pkg/db
./db.go:10:2: undefined: sqlite3
```

**Cause**: CGO is not enabled

**Solution**:
```bash
CGO_ENABLED=1 go build -o scheduler0 ./
```

### Missing gcc/compiler

**Symptoms**:
```
gcc: command not found
```

**Solution** (Ubuntu/Debian):
```bash
sudo apt-get update
sudo apt-get install build-essential
```

**Solution** (Alpine):
```bash
apk add gcc musl-dev
```

---

## Node Startup Issues

### Node Won't Start: "failed to connect to etcd"

**Symptoms**:
```
[ERROR] failed to connect to etcd: connection refused
```

**Cause**: etcd is not running or unreachable

**Solutions**:

1. **Check if etcd is running**:
```bash
curl http://localhost:2379/health
```

2. **Start etcd**:
```bash
etcd --data-dir /tmp/etcd-data.etcd \
     --listen-client-urls http://0.0.0.0:2379 \
     --advertise-client-urls http://localhost:2379
```

3. **Check SCHEDULER0_ETCD_ENDPOINTS**:
```bash
echo $SCHEDULER0_ETCD_ENDPOINTS
# Should output: http://localhost:2379
```

### Node Won't Start: "empty address in configuration"

**Symptoms**:
```
[FATAL] empty address in configuration
```

**Cause**: Using `config.yml` file instead of environment variables

**Solution**: 
1. Remove or rename `config.yml`
2. Set required environment variables:
```bash
export SCHEDULER0_HOST=127.0.0.1
export SCHEDULER0_CLIENT_PORT=9090
export SCHEDULER0_NODE_PORT=8080
export SCHEDULER0_SERVICE_DISCOVERY_HOST=127.0.0.1
export SCHEDULER0_NODE_ID=1
export SCHEDULER0_BOOTSTRAP=true
export SCHEDULER0_ETCD_ENDPOINTS=http://localhost:2379
```

### Node Panics: "invalid secret key length"

**Symptoms**:
```
panic: crypto/aes: invalid key size 10
```

**Cause**: SCHEDULER0_SECRET_KEY is not the correct length

**Solution**: Secret key must be 32, 48, or 64 hex characters (16, 24, or 32 bytes):
```bash
# Generate a valid 32-byte (64 hex char) key
export SCHEDULER0_SECRET_KEY=$(openssl rand -hex 32)
```

### Port Already in Use

**Symptoms**:
```
[FATAL] listen tcp :9090: bind: address already in use
```

**Solutions**:

1. **Find what's using the port**:
```bash
lsof -i :9090
```

2. **Use a different port**:
```bash
export SCHEDULER0_CLIENT_PORT=9091
```

3. **Kill the conflicting process**:
```bash
kill <PID>
```

### Database Initialization Failed

**Symptoms**:
```
[FATAL] failed to initialize database: unable to open database file
```

**Solutions**:

1. **Check directory permissions**:
```bash
ls -la sqlite_data/
# Should be writable by current user
```

2. **Create directory if missing**:
```bash
mkdir -p sqlite_data
```

3. **Re-run init**:
```bash
./scheduler0 init
```

---

## Job Execution Issues

### Jobs Created But Not Executing

**Debug Steps**:

1. **Check if node is leader**:
```bash
curl http://localhost:9090/api/v1/cluster/peers
# Look for "isLeader": true
```

2. **Check job status**:
```bash
curl http://localhost:9090/api/v1/jobs/:id \
  -H "X-API-Key: ..." \
  -H "X-Secret-Key: ..." \
  -H "X-Account-ID: 1"
# status should be "active"
```

3. **Check execution logs**:
```bash
curl http://localhost:9090/api/v1/jobs/:id/executions \
  -H "X-API-Key: ..." \
  -H "X-Secret-Key: ..." \
  -H "X-Account-ID: 1"
```

4. **Check node logs**:
```bash
# Look for errors in scheduler0 output
grep -i error scheduler0.log
```

**Common Causes**:
- Job status is "inactive"
- Cron spec is in the past
- Executor webhook URL is unreachable
- Account quota exceeded

### Webhook Returns 4xx/5xx But Job Marked as Success

**Cause**: Executor error handling

**Solution**: Check execution logs for response codes:
```bash
curl http://localhost:9090/api/v1/jobs/:id/executions
```

Jobs are retried up to `retryMax` times on failure.

### Jobs Executing Multiple Times

**Cause**: At-least-once execution semantics during leader transitions

**Explanation**: This is expected behavior. Scheduler0 guarantees at-least-once execution, not exactly-once.

**Mitigation**: Make your job handlers idempotent using:
- Execution IDs (check if already processed)
- Database transactions
- Timestamps

### Cron Spec Not Parsing

**Symptoms**:
```json
{
  "success": false,
  "message": "invalid cron spec"
}
```

**Common Issues**:
1. **Using 6-field format**: Scheduler0 uses 5-field cron (minute, hour, day, month, weekday)
2. **Invalid characters**: Check for typos
3. **Range errors**: e.g., `60 * * * *` (minutes are 0-59)

**Test your spec**:
```bash
# Use @every for simple intervals
spec="@every 5m"

# Or standard cron (5 fields)
spec="0 9 * * 1-5"  # Weekdays at 9 AM
```

---

## Cluster Issues

### Cannot Join Cluster: "node already exists"

**Symptoms**:
```
[ERROR] failed to join cluster: node ID already in use
```

**Cause**: Node ID conflicts with existing node

**Solution**: Use a unique node ID:
```bash
export SCHEDULER0_NODE_ID=2  # Different from other nodes
```

### Leader Election Takes Too Long

**Symptoms**: Cluster is slow to elect a leader after failure

**Solutions**:

1. **Check network latency** between nodes
2. **Reduce election timeout** (default is sensible for most cases)
3. **Ensure odd number of nodes** (3, 5, or 7)

### Split Brain / Multiple Leaders

**This should not happen** due to Raft's majority quorum requirement.

**If it does occur**:
1. Check for clock skew between nodes
2. Verify network connectivity
3. Review Raft logs for election conflicts
4. **File a bug report** - this is a serious issue

### Node Stuck in "Candidate" State

**Symptoms**: Node repeatedly tries to become leader but fails

**Causes**:
- Network partition
- Less than majority of nodes available
- Misconfigured node addresses

**Solutions**:
1. **Check cluster size**: Need majority online (2 of 3, 3 of 5, etc.)
2. **Verify network connectivity** between nodes
3. **Check node addresses** in etcd registration

---

## Performance Issues

### High CPU Usage

**Common Causes**:
1. **Too many jobs**: Reduce job count or increase cluster size
2. **Aggressive cron specs**: Jobs running every second
3. **Slow executor responses**: Timeout blocking workers

**Solutions**:
```bash
# Check number of active jobs
curl http://localhost:9090/api/v1/jobs?status=active | jq '.data.total'

# Review job specs for overly aggressive schedules
curl http://localhost:9090/api/v1/jobs | jq '.data.jobs[] | select(.spec | contains("* * * * *"))'
```

### High Memory Usage

**Causes**:
1. **Large execution logs**: Old logs not cleaned up
2. **Raft log growth**: Snapshots not being taken
3. **Memory leak**: Report as bug

**Solutions**:
1. **Clean old execution logs** (future feature)
2. **Check snapshot settings**:
   - `RaftSnapshotInterval`: How often to snapshot (seconds)
   - `RaftSnapshotThreshold`: Log entries before snapshot
3. **Monitor memory**: Use system monitoring tools

### Slow API Responses

**Debug**:
1. **Check if node is leader**: Non-leaders forward to leader (extra hop)
2. **Check database size**: Large SQLite DB = slower queries
3. **Check disk I/O**: SQLite performance depends on disk speed

**Solutions**:
- Direct clients to leader node
- Use SSD for better SQLite performance
- Add database indexes (future improvement)

---

## Database Issues

### Database Corruption

**Symptoms**:
```
[ERROR] database disk image is malformed
```

**Solutions**:

1. **Restore from backup**:
```bash
cp sqlite_data/scheduler0.db.backup sqlite_data/scheduler0.db
```

2. **Rebuild from Raft snapshots** (if no backup):
```bash
# Stop node
# Delete SQLite DB
rm sqlite_data/scheduler0.db
# Restart node (will restore from Raft snapshot)
./scheduler0 start
```

### Database Locked

**Symptoms**:
```
[ERROR] database is locked
```

**Causes**:
- Another process has the DB open
- Stale lock file

**Solutions**:
1. **Stop all scheduler0 processes**
2. **Remove WAL files**:
```bash
rm sqlite_data/scheduler0.db-wal
rm sqlite_data/scheduler0.db-shm
```
3. **Restart**

### Raft Log Too Large

**Symptoms**: `raft_data/` directory consuming lots of disk space

**Solutions**:
1. **Verify snapshots are being taken**:
   - Check for `.snapshot` files in `raft_data/`
2. **Adjust snapshot settings** (reduce interval or threshold)
3. **Manual snapshot** (CLI command - future feature)

---

## Getting Help

If you're still stuck:

1. **Check logs** for detailed error messages
2. **Enable debug logging**:
   ```bash
   export SCHEDULER0_LOG_LEVEL=DEBUG
   ```
3. **Search [GitHub Issues](https://github.com/scheduler0/scheduler0/issues)**
4. **Ask in [GitHub Discussions](https://github.com/scheduler0/scheduler0/discussions)**
5. **File a bug report** with:
   - Scheduler0 version
   - Operating system
   - Configuration (sanitize secrets!)
   - Error logs
   - Steps to reproduce

---

**Next**: [Configuration Guide](Configuration) | [Monitoring](Monitoring)
