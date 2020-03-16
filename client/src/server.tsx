require('dotenv').config();

// @ts-ignore
import path from 'path';
// @ts-ignore
import express from 'express';
import helmet from 'helmet';
import morgan from 'morgan';

// @ts-ignore
import bodyParser from "body-parser";
import axios from 'axios';
import { serverRender } from './renderers/server';

import executionsRouter from "./routers/executions";
import credentialRouter from "./routers/credential";
import projectRouter from "./routers/project";
import jobRouter from "./routers/job";

// @ts-ignore
import webpack from 'webpack';
import webpackDevMiddleware from 'webpack-dev-middleware';

const app = express();
const config = require('../webpack.config.js');
const compiler = webpack(config);

const PORT = process.env.PORT || 4323;
const isDev = process.env.NODE_ENV === 'development';
const API_ENDPOINT = process.env.API_ENDPOINT;
const username = process.env.BASIC_AUTH_USER;
const password = process.env.BASIC_AUTH_PASS;

const axiosInstance = axios.create({
    baseURL: API_ENDPOINT,
    auth: {
        username,
        password
    }
});

app.use(webpackDevMiddleware(compiler, {
    publicPath: config.output.publicPath,
    hot: true,
    writeToDisk: true,
    historyApiFallback: true
}));

if (isDev) {
    app.use(require("webpack-hot-middleware")(compiler));
}

app.use('/public', express.static(path.join(__dirname, '/public')));
app.use('/static', express.static(path.join(__dirname, '/static')));
app.use(helmet());
app.use(morgan("combined"));
app.use(bodyParser.json());

app.get("(/|/projects|/jobs|/credentials)", async (req, res) => {
    // TODO: Only make fetch page user is visiting
    const fetchExecutions = axiosInstance.get(`${API_ENDPOINT}/executions`);
    const fetchCredentials = axiosInstance.get(`${API_ENDPOINT}/credentials`);
    const fetchProjects = axiosInstance.get(`${API_ENDPOINT}/projects`);
    const fetchJobs = axiosInstance.get(`${API_ENDPOINT}/jobs`);

    let executions = null;
    let credentials = null;
    let projects = null;
    let jobs = null;

    try {
        [executions, credentials, projects, jobs] = await Promise.all([
            fetchExecutions,
            fetchCredentials,
            fetchProjects,
            fetchJobs
        ]);

        executions = Array.isArray(executions.data.data) ? executions.data.data :  [];
        credentials = Array.isArray(credentials.data.data) ? credentials.data.data : [];
        projects = Array.isArray(projects.data.data) ? projects.data.data : [];
        jobs = Array.isArray(jobs.data.data) ? jobs.data.data : [];
    } catch (e) {
        throw e
    }

    res.send(serverRender({ credentials, projects, jobs, executions }, req.url))
});

app.use('/api/executions', executionsRouter);
app.use('/api/credentials', credentialRouter);
app.use('/api/projects', projectRouter);
app.use('/api/jobs', jobRouter);

app.listen(PORT, () => console.log(`App listening on port ${PORT}`));
