package process

import (
	"testing"
	"time"
)

func TestFixStateJobs(t *testing.T) {
	t.Log("Fix stale job")
	{
		FixStaleJobs()
		time.Sleep(time.Second * 5)
	}
}
