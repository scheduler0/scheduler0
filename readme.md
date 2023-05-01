<img src="./logo.png" height="250" />

# Scheduler0

A cloud-native distributed cron-job server based on Raft distributed consensus and sqlite.

## How It Works

Scheduler0 can run as a single node or a cluster of nodes. As a single node Scheduler0 will handle 
incoming http request to CRUD cron jobs, client credentials, projects and execute the cron jobs. 

However, as a cluster the node that is configured to bootstrap the raft cluster will handle incoming 
http request to CRUD cron jobs, client credentials, projects and automatically load balance the execution of the jobs on 
the other nodes in the cluster. 

When a new leader is elected or changes in the membership of the cluster occurs, 
as long as there's more than one node in the cluster, the execution of jobs will not be handled by the leader. 

When the nodes restart the jobs are queued and continue executions. Scheduler0 can recover jobs that yet to execute upon
a restart, for example if a jobs is supposed to execute every 3days and the cluster or node goes down on the second day, 
if it comes back online before the 3day, the job will still be executed on the expected day.

## Getting Started

Build from source
```shell
go build -o scheduler0
```

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
To start the http server.
```shell
scheduler0 start
```
Use the command below to create an api-key and api-secret for a client. This api-key and secret can be used to make request to any nodes in the cluster.
```shell
scheduler0 create credential 
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
| Protocol | The protocol in which the nodes used to communicate with each other. Only HTTP is supported.                                                                                     |
| Host | The host in which the node can be reached by other nodes                                                                                                                         |
| Port | The port in which the node can be reached by other nodes                                                                                                                         |
| SecretKey | AES256 secret key used for creating api keys and api secrets for client authentication                                                                                           |
| AuthUsername | Username used for basic authentication with other nodes                                                                                                                          |
| AuthPassword | Password used for basic authentication with other nodes                                                                                                                          |
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


## Using Docker Compose 

Here is an example of running a cluster with three nodes using docker compose [https://github.com/iamf-dev/scheduler0-docker/blob/main/cluster_of_three/docker-compose.yml] 
it shows how to set the required environment variables and uses docker volume binds

## Example Usage In Node Server

This is a node client example that creates a project, and 100,000 jobs and counts the number of times the jobs 
get executed.

```javascript
'use strict'

/**
 * The purpose of this program is to test the scheduler0 server.
 * It sends a request scheduler0 server and counts scheduler0 request to the callback url.
 * It's kinda like a scratch pad.
 * **/

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
    timeout: 999999999,
    headers: {
        'x-api-key': scheduler0ApiKey,
        'x-secret-key': scheduler0ApiSecret
    }
});

axiosInstance.interceptors.request.use(request => {
    request.maxContentLength = Infinity;
    request.maxBodyLength = Infinity;
    return request;
})

async function createProject() {
    const { data: { data } } = await axiosInstance
        .post('/projects', {
            name: "sample project",
            description: "my calendar project"
        });
    return data
}

async function createJobs(projectID) {
    let payload = [];

    for (let i = 0; i < 100; i++) {
        for (let j = 0; j < 1000; j++) {
            payload.push({
                spec: "@every 1m",
                project_id: projectID,
                execution_type: "http",
                data: JSON.stringify({jobId: i + j}),
                timezone: 'America/New_York',
                callback_url: `http://localhost:3000/callback`
            })
        }

        try {
            const {data: {data}} = await axiosInstance
                .post('/jobs', payload);
            payload = []
        } catch (err) {
            console.error(err)
        }
    }
}

const hits = new Map();

app.use(express.json({limit: '3mb'}));

app.post('/callback', (req, res) => {

    const payload =  req.body

    res.send(null);

    payload.forEach((payload) => {
        if (!hits.has(payload.id)) {
            hits.set(payload.id, 0);
        }

        hits.set(payload.id, hits.get(payload.id) + 1);
    })

    const hitCounts = new Map();

    const values = hits.values();
    let currentValue = values.next();
    while (currentValue) {
        const { value, done } = currentValue;

        if (done) {
            break;
        }

        if (!hitCounts.has(value)) {
            hitCounts.set(value, 0);
        }

        hitCounts.set(value, hitCounts.get(value) + 1);

        currentValue = values.next();
    }

    const min = Math.min(...Array.from(hits.values()));
    const max = Math.max(...Array.from(hits.values()));

    console.log(hits.size, hitCounts, min, max)
});

app.listen(port, async () => {
    const project = await createProject();
    await createJobs(project.id);
    console.log(`app listening at http://localhost:${port}`);
});
```

## LICENSE

GNU Affero General Public License v3.0