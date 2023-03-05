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
These configurations can only be set in a `config.yml` file next to the scheduler0 binary. 
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