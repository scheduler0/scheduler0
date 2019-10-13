require('dotenv').config();

const path = require("path");
const webpack = require('webpack');
const nodeExternals = require('webpack-node-externals');

module.exports = {
    entry: ['./src/server.tsx'],
    watch: false,
    mode: "production",
    target: "node",
    externals: [nodeExternals()],
    node: {
        __filename: true,
        __dirname: true
    },
    module: {
        rules: [
            {
                test: /\.(js|ts)x?$/,
                use: [
                    {
                        loader: "babel-loader",
                        options: {
                            babelrc: false,
                            presets: [
                                "@babel/env",
                                "@babel/preset-react",
                                "@babel/preset-typescript"
                            ],
                            plugins: [
                                "transform-regenerator",
                                "@babel/plugin-syntax-dynamic-import",
                                ["@babel/plugin-transform-runtime", { useESModules: true }],
                                "transform-class-properties"
                            ]
                        }
                    }
                ],
                exclude: /node_modules/
            },
            {
                test: /\.css$/,
                use: ["style-loader", "css-loader"]
            },
            {
                test: /\.(png|jpg|gif|svg)$/,
                loader: 'url-loader'
            }
        ]
    },

    plugins: [
        new webpack.optimize.OccurrenceOrderPlugin(),
        new webpack.NoEmitOnErrorsPlugin(),
        new webpack.DefinePlugin({
            "process.env": {
                NODE_ENV: JSON.stringify(process.env.NODE_ENV),
                PORT: JSON.stringify(process.env.PORT),
                API_ENDPOINT: JSON.stringify(process.env.API_ENDPOINT),

            }
        }),
    ],

    resolve: {
        extensions: ['.ts', '.tsx', '.js', '.jsx'],
        modules: [
            path.resolve( __dirname, 'src'),
            'node_modules'
        ]
    },

    output: {
        path: path.resolve(__dirname, "build"),
        filename: "server.js",
    },
};