package managers

import (
	"cron-server/server/tests"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	tests.Prepare()
	code := m.Run()
	tests.Teardown()
	os.Exit(code)
}
