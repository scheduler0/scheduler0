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

app.post('/callback', (req, res) => {
    console.log(`Callback executed at :${(new Date()).toUTCString()}`)
    res.send(`Callback executed at :${(new Date()).toUTCString()}`)
})

app.listen(port, async () => {
    // const project = await createProject()
    // const job  = await createJob("930f7772-b3a2-4da2-a775-4a0b9126712c")
    // console.log({ job })
    console.log(`app listening at http://localhost:${port}`)
})
