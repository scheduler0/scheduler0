FROM node:14-alpine

MAINTAINER Victor Nwaokocha

# Create app directory
RUN mkdir -p /usr/src/client
WORKDIR /usr/src/client

# Install app dependencies
COPY package.json /usr/src/client/
RUN npm install

# Bundle app source
COPY src /usr/src/client/src
COPY webpack.config.js /usr/src/client/
COPY config.prod.js /usr/src/client/
COPY tsconfig.json /usr/src/client/
COPY babel-node.config.js /usr/src/client/

ENV PORT $PORT
ENV API_ENDPOINT $API_ENDPOINT
ENV BASIC_AUTH_USER $BASIC_AUTH_USER
ENV BASIC_AUTH_PASS $BASIC_AUTH_PASS

RUN npm run build

EXPOSE $PORT

CMD ["npm", "run", "prod-start"]
