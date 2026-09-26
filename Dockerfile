# syntax=docker/dockerfile:1

FROM golang:1.26 as builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /bin/scheduler0 ./

FROM debian:12-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 curl && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /bin/scheduler0 /bin/scheduler0
EXPOSE 9090
ENV SCHEDULER0_CLIENT_PORT=9090
ENTRYPOINT ["/bin/scheduler0","start"]

