# Use the official Golang image as the base image
FROM golang:1.18

# Set the working directory
WORKDIR /app

# Clone the repository
RUN git clone https://github.com/victorlenerd/scheduler0.git .

# Build the binary
RUN go build -o scheduler0

RUN ./scheduler0 config init

RUN ./scheduler0 credential init

# Expose the default port used by the application
EXPOSE 7071
EXPOSE 9091

# Start the program
CMD ["./scheduler0 start"]