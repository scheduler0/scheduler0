.EXPORT_ALL_VARIABLES:

POSTGRES_ADDRESS = localhost:5432
POSTGRES_DATABASE = scheduler0_test
POSTGRES_USER = core
POSTGRES_PASSWORD = localdev

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

	go test ./src/managers/execution -cover -v -race
	go test ./src/managers/job -cover -v -race
	go test ./src/managers/project -cover -v -race
	go test ./src/managers/credential -cover -v -race

	go test ./src/controllers/execution -cover -v -race
	go test ./src/controllers/credential -cover -v -race
	go test ./src/controllers/job -cover -v -race
	go test ./src/controllers/project -cover -v -race