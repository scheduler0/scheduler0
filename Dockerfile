# Use the official Golang image as the base image
FROM golang:1.18

# Set the working directory
WORKDIR /app

# Copy the contents of the current directory to the working directory
COPY . .

# Build the binary
RUN go build -o scheduler0

# Start the program
CMD ["/app/scheduler0" , "start"]