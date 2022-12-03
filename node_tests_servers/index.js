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
    timeout: 10000,
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

async function createJobs(projectID, name) {
    let payload = [];

    // for (let i = 0; i < 9999999; i++) {
    //     try {
    //         payload.push({
    //             name: name,
    //             spec: "@every 1h30m",
    //             project_id: projectID,
    //             data: JSON.stringify({ jobId: i }),
    //             callback_url: `http://localhost:3000/callback`
    //         })
    //         const { data: { data } } = await axiosInstance
    //             .post('/jobs', payload );
    //         payload = []
    //     } catch (err) {
    //         console.error(err)
    //     }
    // }

    for (let i = 0; i < 999999; i++) {
        if (payload.length ===  999){
            try {
                const { data: { data } } = await axiosInstance
                    .post('/jobs', payload );
                payload = []
            } catch (err) {
                console.error(err)
            }
        } else {
            payload.push({
                name: name,
                spec: "@every 1h30m",
                project_id: projectID,
                data: JSON.stringify({ jobId: i }),
                callback_url: `http://localhost:3001/callback`
            })
        }
    }
}

const hits = new Map();

app.use(express.json());

app.post('/callback', (req, res) => {

    const payload =  req.body

    res.send(null);

    payload.forEach((payload) => {
        if (!hits.has(payload.id)) {
            hits.set(payload.id, 1);
        } else {
            hits.set(payload.id, hits.get(payload.id) + 1);
        }
    })

    console.log(hits)
});

app.listen(port, async () => {
    const project = await createProject();
    await createJobs(project.id, `job_id_`);
    console.log(`app listening at http://localhost:${port}`);
});

// app.listen(port);
