.EXPORT_ALL_VARIABLES:

POSTGRES_ADDRESS = localhost:5432
POSTGRES_DATABASE = scheduler0_test
POSTGRES_USER = core
POSTGRES_PASSWORD = localdev

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
	docker run --rm -it --name postgres -p 5432:5432 \
		-e POSTGRES_USER=core \
		-e POSTGRES_DB=scheduler0_test \
		-e POSTGRES_PASSWORD=localdev \
		postgres:13-alpine

clean_test_cache:
	go clean -testcache

test:
	go clean -testcache

	go test ./server/managers/execution -cover -v -race
	go test ./server/managers/job -cover -v -race
	go test ./server/managers/project -cover -v -race
	go test ./server/managers/credential -cover -v -race

	go test ./server/http_server/controllers/execution -cover -v -race
	go test ./server/http_server/controllers/credential -cover -v -race
	go test ./server/http_server/controllers/job -cover -v -race
	go test ./server/http_server/controllers/project -cover -v -race

	go test ./server/http_server/middlewares/auth/ios
	go test ./server/http_server/middlewares/auth/android
	go test ./server/http_server/middlewares/auth/server
	go test ./server/http_server/middlewares/auth/web