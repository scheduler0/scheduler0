# Cron Server

Fault tolerant cron job server.

## Dependencies

    1: Postgres
    2: Redis
    3: Rabbitmq

## Installation
    
    1: Build container using host network  `docker build . --tag="cron-server:latest" --network="host"`
    2: Run container `docker run -d -p 8080:8080 cron-server`