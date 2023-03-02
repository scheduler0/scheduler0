# Scheduler0

A Cloud-Native Distributed Cronjob Server based on Raft distributed consensus and sqlite.

## Basic Setup

```shell
scheduler0 config init
```

Will take you through the configuration flow, where you will be prompted to enter your database credentials.

### To start the http server.

```shell
scheduler0 start
```

For more information. Use the help flag
```shell
scheduler0 --help
```

## Configurations

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

These configurations can set in the environment or `config.yml` in the root or by executing:

```shell
scheduler0 config init
```

## Credentials

Credentials are basically api keys and secrets needed for client apps to reach the scheduler0 server. 
Use command to generate credentials and more.

```shell
scheduler0 create credential
```

The above command will create a credential for server, this includes api key and secret used in the node example app below.
You can use the list command to view all credentials.

```shell
scheduler0 list -t credentials
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