# Scheduler0

A cloud-native distributed cron-job server based on Raft distributed consensus and sqlite.

## Basic Setup

Initialize the configuration
```shell
scheduler0 config init
```
Creates the secrets or credentials need for the nodes to authenticate with each other. 
The nodes use basic http authentication, so you have to provide a username, password and a secret key used to generate 
api-key and api-secret for clients. The provided username, password and secret will be stored locally in a .scheduler0 file.
```shell
scheduler0 credential init
```
Use the command below to create an api-key and api-secret for a client. This api-key and secret can be used to make request to any nodes in the cluster.
```shell
scheduler0 create credential 
```
To start the http server.
```shell
scheduler0 start
```
For more information. Use the help flag
```shell
scheduler0 --help
```

## Configurations
These configurations should be set in a `config.yml` file next to the scheduler0 binary. 
```shell
LogLevel: DEBUG
Protocol: http
Host: 127.0.0.1
Port: 9091
MaxMemory: 5000
MaxCPU: 50
Bootstrap: true
NodeId: 1
RaftAddress: 127.0.0.1:7071
RaftTransportMaxPool: 100
RaftTransportTimeout: 1
RaftApplyTimeout: 2
RaftSnapshotInterval: 1
RaftSnapshotThreshold: 1
PeerConnectRetryMax: 2
PeerConnectRetryDelay: 1
PeerAuthRequestTimeoutMs: 2
JobExecutionTimeout: 30
JobExecutionRetryDelay: 1
JobExecutionRetryMax: 10
MaxWorkers: 1
MaxQueue: 1
ExecutionLogFetchFanIn: 2
ExecutionLogFetchIntervalSeconds: 2
HTTPExecutorPayloadMaxSizeMb: 2
Replicas:
  - Address: http://127.0.0.1:9091
    RaftAddress: 127.0.0.1:7071
    NodeId: 1
  - Address: http://127.0.0.1:9092
    RaftAddress: 127.0.0.1:7072
    NodeId: 2
  - Address: http://127.0.0.1:9093
    RaftAddress: 127.0.0.1:7073
    NodeId: 3
```

| Config       | Description                                                                                                                                                                      |
|--------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| LogLevel | Log level can be ERROR, DEBUG, INFO or WARN                                                                                                                                      |
| Protocol | The protocol in which the nodes used to communicate with each other. Only HTTP is supported for now                                                                              |
| Host | The host in which the node can be reached by other nodes                                                                                                                         |
| Port | The port in which the node can be reached by other nodes                                                                                                                         |
| MaxMemory | This restricts the how much memory is consumed. Once 70% of the memory specified is reached scheduler0 will stop scheduling jobs on the node and stop executing jobs on the node |
| Bootstrap | If set to true this node will startup the cluster. Only a single node should have this set to true
| NodeId | The id of the node in the cluster. It should be unique for the node.
| RaftAddress | The network address for the node dedicated for raft communication purposes
| RaftTransportMaxPool | Maximum number of connection pool raft should use
| RaftTransportTimeout | The raft connection timeout
| RaftApplyTimeout | Timeout for replicating data across nodes
| RaftSnapshotInterval | SnapshotInterval controls how often we check if we should perform a snapshot.
| RaftSnapshotThreshold | SnapshotThreshold controls how many outstanding logs there must be before we perform a snapshot.
| PeerConnectRetryMax | Number of times to retry connection to other nodes
| PeerConnectRetryDelay | Delay between each retry attempt to connect to other nodes
| PeerAuthRequestTimeoutMs | Authentication request time out in seconds
| JobExecutionTimeout | How long to wait for a job to complete execution before considering it a failed job
| JobExecutionRetryDelay | How long to wait before retrying a failed job execution
| JobExecutionRetryMax | Maximum number of times to retry executing job
| MaxWorkers | The number of workers to execute jobs and create new jobs
| MaxQueue | The size of the queues in which workers pick jobs from
| ExecutionLogFetchFanIn | Number of nodes to fetch local execution logs at a time
| ExecutionLogFetchIntervalSeconds | Time between each attempt to fetch local job execution logs
| HTTPExecutorPayloadMaxSizeMb | Maximum size of payload to send to client expecting job execution
| Replicas | A list of all the nodes in the replicas and their addresses


## Example Usage In Node Server

```javascript
'use strict'

require('dotenv').config()

const axios = require('axios')
const express = require('express')

const app = express()
const port = 3000

// Scheduler0 environment variables
const scheduler0Endpoint = process.env.API_ENDPOINT
const scheduler0ApiKey = process.env.API_KEY
const scheduler0ApiSecret = process.env.API_SECRET

const axiosInstance = axios.create({
    baseURL: scheduler0Endpoint,
    headers: {
        'x-api-key': scheduler0ApiKey,
        'x-secret-key': scheduler0ApiSecret
    }
});

async function createProject() {
    const { data: { data } } = await axiosInstance
        .post('/projects', {
            name: "sample project",
            description: "my calendar project"
        });
    return data
}

async function createJob(projectUUID) {
    try {
        const { data: { data } } = await axiosInstance
            .post('/jobs', {
                name: "sample project",
                spec: "@every 1m",
                project_uuid: projectUUID,
                callback_url: "http://localhost:3000/callback"
            });

        console.log(data)

        return data
    } catch (err) {
        console.log({ error: err.response.data })
    }
}

// This callback will get executed every minute
app.post('/callback', (req, res) => {
    res.send(`Callback executed at :${(new Date()).toUTCString()}`)
})

app.listen(port, async () => {
    const project = await createProject()
    const job  = await createJob(project.uuid)
   
    console.log(`app listening at http://localhost:${port}`)
})

```