# Use the official Golang image as the base image
FROM golang:1.18

# Set the working directory
WORKDIR /app


RUN git clone https://github.com/stedolan/jq.git
RUN cd jq
RUN autoreconf -i
RUN ./configure --disable-maintainer-mode
RUN make
RUN sudo make install

# Clone the repository
RUN git clone https://github.com/victorlenerd/scheduler0.git .

# Build the binary
RUN go build -o scheduler0

RUN ./scheduler0 config init

RUN output=$(./scheduler0 create credential)

# Extract the API key and API secret from the output
RUN api_key=$(echo $output | jq -r '.data.api_key')
RUN api_secret=$(echo $output | jq -r '.data.api_secret')

RUN rm -rf .env
RUN touch .env

# Write the API key and API secret to the .env file
RUN echo "API_KEY=$api_key" >> .env
RUN echo "API_SECRET=$api_secret" >> .env

# Expose the default port used by the application
EXPOSE 7071
EXPOSE 9091

# Start the program
CMD ["./scheduler0 start"]