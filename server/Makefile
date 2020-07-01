prepare_test_db:
	go run ./testutils/testutils.go

test_all:
	go test ./... -cover -v

test: prepare_test_db test_all