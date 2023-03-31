# Use the official Golang image as the base image
FROM golang:1.18

# Set the working directory
WORKDIR /app

# Clone the repository
RUN git clone https://github.com/victorlenerd/scheduler0.git .

# Build the binary
RUN go build -o scheduler0

RUN ./scheduler0 config init

RUN echo '{"secretKey":"AB551DED82B93DC8035D624A625920E2121367C7538C02277D2D4DB3C0BFFE94","authUsername":"admin","authPassword":"admin"}' >> .scheduler0

# Expose the default port used by the application
EXPOSE 7070
EXPOSE 9090

# Start the program
CMD ["/app/scheduler0" , "start"]