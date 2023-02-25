#!/bin/bash

kill -9 $(lsof -ti:3000)

# Execute the command and store the output in a variable
output=$(./scheduler0 create credential)

# Extract the API key and API secret from the output
api_key=$(echo $output | jq -r '.data.api_key')
api_secret=$(echo $output | jq -r '.data.api_secret')

rm -rf .env
touch .env

# Write the API key and API secret to the .env file
echo "API_KEY=$api_key" >> .env
echo "API_SECRET=$api_secret" >> .env
echo "API_ENDPOINT=http://localhost:9090" >> .env

node node_tests_servers/index

echo "API key and API secret written to .env file"