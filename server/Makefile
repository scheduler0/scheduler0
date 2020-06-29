prepare_test_db:
	go run prepare_test_db.go

teardown_test_db:
	go run teardown_test_db.go

test: teardown_test_db prepare_test_db
	 go test ./... -cover -v