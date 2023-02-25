#!/bin/bash

go build -o scheduler0
kill -9 $(lsof -ti:7070)
./scheduler0 config init
./scheduler0 start