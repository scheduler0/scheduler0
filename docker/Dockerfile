# Use the official Golang image as the base image
FROM golang:1.18

# Set the working directory
WORKDIR /app

# Clone the repository
RUN git clone https://github.com/victorlenerd/scheduler0.git .

# Build the binary
RUN go build -o scheduler0

RUN rm rf config.yml
RUN rm rf .scheduler0

RUN ./scheduler0 config init

# Expose the default port used by the application
EXPOSE 7070
EXPOSE 9090

# Start the program
CMD ["/app/scheduler0" , "start"]