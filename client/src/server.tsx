require('dotenv').config()

import path from 'path'
import express from 'express'
import helmet from 'helmet'
import morgan from 'morgan'
import { serverRender } from './renderers/server'

import webpack from 'webpack';
import webpackDevMiddleware from 'webpack-dev-middleware'

const app = express()
const config = require('../webpack.config.js');
const compiler = webpack(config);

const PORT = process.env.PORT || 4323;
const isDev = process.env.NODE_ENV === 'development';

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

app.get('/', (req, res) => {
    res.send(serverRender({}))
});

app.listen(PORT, () => console.log(`App listening on port ${PORT}`));