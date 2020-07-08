package controllers

import (
	"cron-server/server/tests"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	tests.Teardown()
	tests.Prepare()
	code := m.Run()
	os.Exit(code)
}