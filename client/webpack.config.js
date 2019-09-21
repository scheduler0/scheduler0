require('dotenv').config()

const path = require("path")
const webpack = require('webpack');

module.exports = {
    entry: process.env.NODE_ENV === 'development' ? [
        "webpack-hot-middleware/client?path=http://localhost:"+process.env.PORT+"/__webpack_hmr",
        "./src/client.tsx",
    ] : ["./src/client.tsx"],
    watch: process.env.NODE_ENV === 'development',
    mode: process.env.NODE_ENV,
    devtool: "source-map",
    target: "web",
    module: {
        rules: [
            {
                test: /\.(js|ts)x?$/,
                use: [
                    "react-hot-loader/webpack",
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
                                "react-hot-loader/babel",
                                "transform-regenerator",
                                "@babel/plugin-syntax-dynamic-import",
                                ["@babel/plugin-transform-runtime", { useESModules: true }],
                                "transform-class-properties"
                            ]
                        }
                    }
                ],
                exclude: /node_modules/
            }
        ]
    },

    plugins: [
        new webpack.optimize.OccurrenceOrderPlugin(),
        new webpack.HotModuleReplacementPlugin(),
        new webpack.NoEmitOnErrorsPlugin(),
        new webpack.DefinePlugin({
            "process.env": {
                NODE_ENV: JSON.stringify(process.env.NODE_ENV),
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
        path: path.resolve(__dirname, "src/public/dist"),
        filename: "bundle.js",
        publicPath: '/public/dist/'
    },
}