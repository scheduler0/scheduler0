# Scheduler0

A simple scheduling server for apps and backend server.

[![Go Report Card](https://goreportcard.com/badge/github.com/victorlenerd/scheduler0)](https://goreportcard.com/report/github.com/victorlenerd/scheduler0) 
[![CircleCI](https://circleci.com/gh/victorlenerd/scheduler0/tree/master.svg?style=svg)](https://circleci.com/gh/victorlenerd/scheduler0/tree/master)

## Current Status

To use this you must first clone the repository and add the scheduler0 binary at the root of repo to your $PATH
    
The scheduler0 CLI tool in the root of this repo can be used to configure and start the server.
Postgres is needed to start the server, therefore after cloning the repo you need to run the config command.
```shell
scheduler0 config init
```

Will take you through the configuration flow, where you will be prompted to enter your database credentials.
To start the http server.
```shell
scheduler0 start
```

For more information. Use the help flag
```shell
scheduler0 --help
```

## API Documentation

There is a REST API documentation on http://localhost:9090/api-docs/ [](http://localhost:9090/api-docs/)

Note: that port 9090 is the default port for the server.

## [WIP]: Dashboard

The dashboard is supposed to be a GUI for managing projects, credentials and jobs.

!["Dashboard"](./screenshots/screenshot.png)
 
# License

 * MIT license ([LICENSE-MIT](LICENSE-MIT) or
   http://opensource.org/licenses/MIT)

at your option.

### Contribution

Unless you explicitly state otherwise, any contribution intentionally submitted
for inclusion in this project by you, as defined in the Apache-2.0 license,
shall be dual licensed as above, without any additional terms or conditions.
