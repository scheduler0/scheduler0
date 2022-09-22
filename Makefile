.EXPORT_ALL_VARIABLES:

POSTGRES_ADDRESS = localhost:5432
POSTGRES_DATABASE = scheduler0_test
POSTGRES_USER = core
POSTGRES_PASSWORD = localdev

SCHEDULER0_POSTGRES_ADDRESS = localhost:5432
SCHEDULER0_POSTGRES_PASSWORD = localdev
SCHEDULER0_POSTGRES_DATABASE = scheduler0_test
SCHEDULER0_POSTGRES_USER = core
SCHEDULER0_SECRET_KEY=2223

build:
	go build scheduler0.go

test_cli:
	go build scheduler0.go
	./scheduler0 start

circle_ci_yaml_validation:
	brew upgrade circleci

execute_circle_ci_job:
	circleci local execute --job build

build_server_test_dockerfile:
	docker build --file docker/server/Dockerfile.server-test \
		--build-arg PORT=4321 \
		--build-arg POSTGRES_ADDRESS=localhost:5432 \
		--build-arg POSTGRES_DATABASE=scheduler0_test \
		--build-arg POSTGRES_USER=core \
		--build-arg POSTGRES_PASSWORD=localdev \
		--build-arg BASIC_AUTH_USER=admin \
		--build-arg BASIC_AUTH_PASS=admin  \
		.

start_test_db:
	cd docker/postgres
	docker build -t scheduler_0_postgres .
	docker run -dp 5432:5432 scheduler_0_postgres

stop_test_db:
	docker container stop postgres

clean_test_cache:
	go clean -testcache

test:
	go clean -testcache

	go test ./server/managers/execution

	go test ./server/managers/job
	go test ./server/managers/project

	go test ./server/http_server/controllers/execution
	go test ./server/http_server/controllers/credential
	go test ./server/http_server/controllers/job
	go test ./server/http_server/controllers/project

	go test ./server/http_server/middlewares/auth/ios
	go test ./server/http_server/middlewares/auth/android
	go test ./server/http_server/middlewares/auth/server
	go test ./server/http_server/middlewares/auth/web

	go test ./server/process

copy_build_into_e2e:
	cp scheduler0 ./e2e/node1/
	cp scheduler0 ./e2e/node2/
	cp scheduler0 ./e2e/node3/
	cp scheduler0 ./e2e/node4/
	cp scheduler0 ./e2e/node5/
	cp scheduler0 ./e2e/node6/
	cp scheduler0 ./e2e/node7/

start_e2e_server1:
	./e2e/node1/scheduler0 config init
	rm -rfd ./e2e/node1/raft_data
	rm -rfd ./raft_data/1
	./e2e/node1/scheduler0 start

start_e2e_server2:
	./e2e/node2/scheduler0 config init
	rm -rfd ./e2e/node2/raft_data
	rm -rfd ./raft_data/2
	./e2e/node2/scheduler0 start

start_e2e_server3:
	./e2e/node3/scheduler0 config init
	rm -rfd ./e2e/node3/raft_data
	rm -rfd ./raft_data/3
	./e2e/node3/scheduler0 start

start_e2e_server4:
	./e2e/node4/scheduler0 config init
	rm -rfd ./e2e/node4/raft_data
	rm -rfd ./raft_data/4
	./e2e/node4/scheduler0 start

start_e2e_server5:
	rm -rfd ./e2e/node5/raft_data
	rm -rfd ./raft_data/5
	./e2e/node5/scheduler0 start

local_raft_test_build:
	docker-compose -f ./docker/docker-compose.yml up --force-recreate --build server

local_raft_test_up:
	docker-compose -f ./docker/docker-compose.yml up server