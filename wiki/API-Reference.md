# API Reference

Complete REST API documentation for Scheduler0.

## Base URL

```
http://<scheduler0-host>:<client-port>/api/v1
```

Default: `http://localhost:9090/api/v1`

## Authentication

### Client Authentication

All client API requests require these headers:

```http
X-API-Key: sk_...
X-Secret-Key: ssk_...
X-Account-ID: 1
```

### Peer Authentication

Node-to-node and CLI requests require:

```http
Authorization: Basic base64(username:password)
X-Peer: cmd
X-Account-ID: 1
```

## Common Response Format

```json
{
  "success": true,
  "message": "optional message",
  "data": { ... }
}
```

## Endpoints

### Health & Status

#### Check Health
```http
GET /api/v1/healthcheck
```

**Response**:
```json
{
  "status": "ok",
  "version": "1.0.0",
  "nodeId": 1
}
```

---

### Credentials

#### Create Credential
```http
POST /api/v1/credentials
Authorization: Basic ...
X-Peer: cmd
X-Account-ID: 1
```

**Request**:
```json
{
  "createdBy": "admin",
  "scopes": ["read", "write"]
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "id": 1,
    "apiKey": "sk_...",
    "secretKey": "ssk_...",
    "accountId": 1,
    "scopes": ["read", "write"],
    "dateCreated": "2026-09-26T05:20:00Z"
  }
}
```

---

### Projects

#### Create Project
```http
POST /api/v1/projects
X-API-Key: sk_...
X-Secret-Key: ssk_...
X-Account-ID: 1
```

**Request**:
```json
{
  "name": "My Project",
  "description": "Project description"
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "id": 1,
    "name": "My Project",
    "description": "Project description",
    "accountId": 1,
    "dateCreated": "2026-09-26T05:20:00Z"
  }
}
```

#### List Projects
```http
GET /api/v1/projects?offset=0&limit=50
X-API-Key: sk_...
X-Secret-Key: ssk_...
X-Account-ID: 1
```

#### Get Project
```http
GET /api/v1/projects/:id
```

#### Update Project
```http
PUT /api/v1/projects/:id
```

**Request**:
```json
{
  "name": "Updated Name",
  "description": "Updated description"
}
```

#### Delete Project
```http
DELETE /api/v1/projects/:id
```

---

### Executors

#### Create Executor
```http
POST /api/v1/executors
```

**Webhook Executor**:
```json
{
  "executorType": "webhook_url",
  "webhookUrl": "https://example.com/webhook"
}
```

**AWS Lambda Executor**:
```json
{
  "executorType": "aws_lambda",
  "functionName": "my-function",
  "functionRegion": "us-east-1",
  "accessKeyId": "AKIA...",
  "secretAccessKey": "..."
}
```

**Azure Function Executor**:
```json
{
  "executorType": "azure_function",
  "functionUrl": "https://myapp.azurewebsites.net/api/function",
  "functionKey": "..."
}
```

**GCP Cloud Function Executor**:
```json
{
  "executorType": "gcp_function",
  "functionUrl": "https://us-central1-project.cloudfunctions.net/function",
  "serviceAccountKey": "{...}"
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "id": 1,
    "executorType": "webhook_url",
    "webhookUrl": "https://example.com/webhook",
    "accountId": 1,
    "dateCreated": "2026-09-26T05:20:00Z"
  }
}
```

#### List Executors
```http
GET /api/v1/executors?offset=0&limit=50
```

#### Get Executor
```http
GET /api/v1/executors/:id
```

#### Delete Executor
```http
DELETE /api/v1/executors/:id
```

---

### Jobs

#### Create Job(s)
```http
POST /api/v1/jobs
```

**Request** (array of jobs):
```json
[
  {
    "projectId": 1,
    "executorId": 1,
    "spec": "@every 5m",
    "data": "{\"key\": \"value\"}",
    "timezone": "America/New_York",
    "retryMax": 3,
    "createdBy": "admin"
  }
]
```

**Cron Spec Formats**:
- `@every 5m` - Every 5 minutes
- `@hourly` - Every hour
- `@daily` - Every day at midnight
- `@weekly` - Every Sunday at midnight
- `0 9 * * 1-5` - Weekdays at 9 AM
- `*/15 * * * *` - Every 15 minutes
- `0 0 1 * *` - First day of month at midnight

**Response**:
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "projectId": 1,
      "executorId": 1,
      "spec": "@every 5m",
      "data": "{\"key\": \"value\"}",
      "timezone": "America/New_York",
      "retryMax": 3,
      "status": "active",
      "accountId": 1,
      "dateCreated": "2026-09-26T05:20:00Z",
      "createdBy": "admin"
    }
  ]
}
```

#### List Jobs
```http
GET /api/v1/projects/:projectId/jobs?offset=0&limit=50&orderBy=dateCreated&orderDirection=desc
```

**Query Parameters**:
- `offset` - Pagination offset (default: 0)
- `limit` - Page size (default: 50, max: 100)
- `orderBy` - Sort column (dateCreated, spec, status)
- `orderDirection` - Sort direction (asc, desc)

#### Get Job
```http
GET /api/v1/jobs/:id
```

#### Update Job
```http
PUT /api/v1/jobs/:id
```

**Request**:
```json
{
  "spec": "@every 10m",
  "data": "{\"updated\": true}",
  "status": "active"
}
```

#### Delete Job
```http
DELETE /api/v1/jobs/:id
```

#### Get Job Executions
```http
GET /api/v1/jobs/:id/executions?offset=0&limit=50
```

**Response**:
```json
{
  "success": true,
  "data": {
    "total": 100,
    "offset": 0,
    "limit": 50,
    "executions": [
      {
        "id": 1,
        "jobId": 1,
        "status": "success",
        "executionTime": "2026-09-26T05:20:00Z",
        "responseCode": 200,
        "responseBody": "OK",
        "executorType": "webhook_url"
      }
    ]
  }
}
```

---

### AI Features

#### Classify Schedule (Natural Language to Cron)
```http
POST /api/v1/ai/classify
```

**Request**:
```json
{
  "prompt": "every weekday at 9am",
  "timezone": "America/New_York"
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "cronSpec": "0 9 * * 1-5",
    "timezone": "America/New_York",
    "explanation": "Runs at 9:00 AM on weekdays (Monday-Friday)",
    "confidence": 0.95
  }
}
```

#### Suggest Schedule Optimization
```http
POST /api/v1/ai/suggest
```

**Request**:
```json
{
  "currentSpec": "* * * * *",
  "jobDescription": "Send daily report email",
  "timezone": "America/New_York"
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "suggestedSpec": "0 8 * * *",
    "reasoning": "Sending every minute is excessive for a daily report. Recommend 8 AM daily.",
    "estimatedSavings": "99.9% reduction in executions"
  }
}
```

---

### Cluster Management

#### List Peers
```http
GET /api/v1/cluster/peers
Authorization: Basic ...
X-Peer: cmd
```

**Response**:
```json
{
  "success": true,
  "data": {
    "leader": {
      "nodeId": 1,
      "address": "http://127.0.0.1:9090",
      "raftAddress": "127.0.0.1:8080",
      "isLeader": true,
      "isHealthy": true
    },
    "peers": [
      {
        "nodeId": 2,
        "address": "http://127.0.0.1:9091",
        "raftAddress": "127.0.0.1:8081",
        "isLeader": false,
        "isHealthy": true
      }
    ]
  }
}
```

#### Transfer Leadership
```http
POST /api/v1/cluster/transfer-leadership?targetNodeId=2
Authorization: Basic ...
X-Peer: cmd
```

#### Remove Node
```http
DELETE /api/v1/cluster/peers/:nodeId
Authorization: Basic ...
X-Peer: cmd
```

---

### Account Management

#### Get Account
```http
GET /api/v1/accounts/:id
```

#### Get Account Quota
```http
GET /api/v1/accounts/:id/quota
```

**Response**:
```json
{
  "success": true,
  "data": {
    "accountId": 1,
    "jobExecutionLimit": 10000,
    "jobExecutionUsed": 5432,
    "jobExecutionRemaining": 4568,
    "aiCreditLimit": 1000,
    "aiCreditUsed": 123,
    "aiCreditRemaining": 877
  }
}
```

---

## Error Responses

### Standard Error Format

```json
{
  "success": false,
  "message": "Error description",
  "error": "ERROR_CODE"
}
```

### Common Error Codes

| Status | Code | Description |
|--------|------|-------------|
| 400 | BAD_REQUEST | Invalid request body or parameters |
| 401 | UNAUTHORIZED | Missing or invalid authentication |
| 403 | FORBIDDEN | Insufficient permissions |
| 404 | NOT_FOUND | Resource not found |
| 409 | CONFLICT | Resource already exists |
| 422 | UNPROCESSABLE_ENTITY | Validation failed |
| 429 | TOO_MANY_REQUESTS | Rate limit exceeded |
| 500 | INTERNAL_ERROR | Server error |
| 503 | SERVICE_UNAVAILABLE | Node not ready or not leader |

### Example Error Response

```json
{
  "success": false,
  "message": "Job spec is invalid: cron expression parse error",
  "error": "INVALID_SPEC"
}
```

---

## Rate Limits

- **Default**: 1000 requests/minute per API key
- **Burst**: Up to 100 requests/second
- **Headers** (included in responses):
  - `X-RateLimit-Limit`: Rate limit
  - `X-RateLimit-Remaining`: Remaining requests
  - `X-RateLimit-Reset`: Reset timestamp

---

## Pagination

List endpoints support pagination:

```http
GET /api/v1/jobs?offset=0&limit=50
```

**Parameters**:
- `offset` - Number of items to skip (default: 0)
- `limit` - Page size (default: 50, max: 100)

**Response** includes pagination metadata:
```json
{
  "success": true,
  "data": {
    "total": 500,
    "offset": 0,
    "limit": 50,
    "jobs": [...]
  }
}
```

---

## Webhooks

### Job Invocation Payload

When a job executes, Scheduler0 POSTs to the executor's webhook URL:

**Single Job**:
```json
{
  "job": {
    "id": 1,
    "projectId": 1,
    "spec": "@every 5m",
    "data": "{\"key\": \"value\"}",
    "timezone": "America/New_York",
    "accountId": 1
  },
  "lastExecutionDateTime": "2026-09-26T05:15:00Z",
  "lastExecutionStatus": "success"
}
```

**Aggregated Jobs** (when payload aggregation is enabled):
```json
{
  "aggregated": true,
  "jobs": [
    {
      "job": { ... },
      "lastExecutionDateTime": "...",
      "lastExecutionStatus": "success"
    },
    {
      "job": { ... },
      "lastExecutionDateTime": "...",
      "lastExecutionStatus": "success"
    }
  ]
}
```

**Expected Response**:
- Status Code: 200-299 for success
- Body: Any (logged for debugging)

---

**Next**: [Job Management](Job-Management) | [Executors Guide](Executors)
