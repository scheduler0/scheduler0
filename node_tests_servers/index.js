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

    // for (let i = 0; i < 9999999; i++) {
    //     payload.push({
    //         spec: "@every 1h30m",
    //         project_id: projectID,
    //         data: JSON.stringify({ jobId: i }),
    //         callback_url: `http://localhost:3000/callback`
    //     })
    //     if (payload.length > 999999) {
    //         try {
    //             const { data: { data } } = await axiosInstance
    //                 .post('/jobs', payload );
    //             payload = []
    //         } catch (err) {
    //             console.error(err)
    //         }
    //     }
    // }

    for (let i = 0; i < 1000; i++) {
        for (let j = 0; j < 1000; j++) {
            payload.push({
                spec: "@every 1m",
                project_id: projectID,
                execution_type: "http",
                data: JSON.stringify({jobId: i + j}),
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

// app.listen(port);


// async function create() {
//     await createJobs(1);
// }

// create().then(() => {
//     console.log("success");
// }).catch((err) => {
//     console.error(err);
// })