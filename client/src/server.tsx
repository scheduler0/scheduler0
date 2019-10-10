require('dotenv').config()

import path from 'path'
import express from 'express'
import helmet from 'helmet'
import morgan from 'morgan'
import axios from 'axios'
import { serverRender } from './renderers/server'

import webpack from 'webpack';
import webpackDevMiddleware from 'webpack-dev-middleware'

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
app.use(helmet());
app.use(morgan("combined"));

app.get('/', async (req, res) => {
    const fetchCredentials = axiosInstance.get(`${API_ENDPOINT}/credentials`);
    const fetchProjects = axiosInstance.get(`${API_ENDPOINT}/projects`);
    const fetchJobs = axiosInstance.get(`${API_ENDPOINT}/jobs`);

    let credentials = null;
    let projects = null;
    let jobs = null;

    try {
        [credentials, projects, jobs] = await Promise.all([
            fetchCredentials,
            fetchProjects,
            fetchJobs
        ]);

        credentials = credentials.data;
        projects = projects.data;
        jobs = jobs.data;
    } catch (e) {
        console.error(e)
    }

    res.send(serverRender({ credentials, projects, jobs }))
});

app.listen(PORT, () => console.log(`App listening on port ${PORT}`));