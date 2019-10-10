module.exports = {
    presets: [
        '@babel/env',
        '@babel/preset-react',
        '@babel/preset-typescript',
    ],
    plugins: [
        "transform-regenerator",
        "@babel/plugin-syntax-dynamic-import",
        ["@babel/plugin-transform-runtime", { useESModules: true }],
        "transform-class-properties",
        ["@babel/plugin-proposal-decorators", { "legacy": true }],
        ["@babel/plugin-proposal-class-properties", { "loose": true }]
    ],
};