FROM golang:1.13

MAINTAINER Victor Nwaokocha

RUN mkdir app
WORKDIR app

ARG PORT
ARG POSTGRES_ADDRESS
ARG POSTGRES_PASSWORD
ARG POSTGRES_DATABASE
ARG REDIS_ADDRESS

COPY . .

RUN go test ./... -cover -v
RUN go build

RUN chmod +x ./cron-server

EXPOSE 8080

ENTRYPOINT ./cron-server