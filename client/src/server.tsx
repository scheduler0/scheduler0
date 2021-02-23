require('dotenv').config({ path: '../.env' });

import path from 'path';
import express from 'express';
import helmet from 'helmet';
import morgan from 'morgan';

import bodyParser from "body-parser";
import axios from 'axios';
import { serverRender } from './renderers/server';

import executionsRouter from "./routers/executions";
import credentialRouter from "./routers/credential";
import projectRouter from "./routers/project";
import jobRouter from "./routers/job";

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

console.log({ API_ENDPOINT })

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
    const fetchCredentials = axiosInstance.get(`credentials?limit=100&offset=0`);
    const fetchProjects = axiosInstance.get(`projects?limit=100&offset=0`);

    let credentials = null;
    let projects = null;

    try {
        [credentials, projects] = await Promise.all([
            fetchCredentials,
            fetchProjects,
        ]);
    } catch (e) {
        console.error(e?.response?.data)
        console.error(e?.stack)
    }

    res.send(serverRender({ credentials: credentials?.data ?? {}, projects: projects?.data ?? {}}, req.url))
});

app.use('/api/executions', executionsRouter);
app.use('/api/credentials', credentialRouter);
app.use('/api/projects', projectRouter);
app.use('/api/jobs', jobRouter);

app.listen(PORT, () => console.log(`App listening on port ${PORT}`));
