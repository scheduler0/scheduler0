# Use the official Golang image as the base image
FROM golang:1.18

# Set the working directory
WORKDIR /app

# Clone the repository
RUN git clone https://github.com/victorlenerd/scheduler0.git .

# Build the binary
RUN go build -o scheduler0

RUN cat << EOF > config.yml \
    LogLevel: DEBUG \
    Protocol: http \
    Host: 127.0.0.1 \
    Port: 9091 \
    MaxMemory: 5000 \
    Bootstrap: true \
    NodeId: 1 \
    RaftAddress: 127.0.0.1:7071 \
    RaftTransportMaxPool: 100 \
    RaftTransportTimeout: 1 \
    RaftApplyTimeout: 2 \
    RaftSnapshotInterval: 1 \
    RaftSnapshotThreshold: 1 \
    PeerConnectRetryMax: 2 \
    PeerConnectRetryDelay: 1 \
    PeerAuthRequestTimeoutMs: 2 \
    JobExecutionTimeout: 30 \
    JobExecutionRetryDelay: 1 \
    JobExecutionRetryMax: 10 \
    MaxWorkers: 1 \
    MaxQueue: 1 \
    ExecutionLogFetchFanIn: 2 \
    ExecutionLogFetchIntervalSeconds: 2 \
    HTTPExecutorPayloadMaxSizeMb: 2 \
    Replicas: \
      - Address: http://127.0.0.1:9091 \
        RaftAddress: 127.0.0.1:7071 \
        NodeId: 1 \

RUN ./scheduler0 config init

RUN ./scheduler0 credential init

# Expose the default port used by the application
EXPOSE 7071
EXPOSE 9091

# Start the program
CMD ["./scheduler0 start"]