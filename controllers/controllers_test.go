package controllers

import (
	"testing"
	"time"
)

func TestDate(t *testing.T) {

	t.Log("Dates")
	{
		startDate := time.Now()
		endDate := startDate.Add(time.Minute)

		println("Diff seconds", int64(endDate.Sub(startDate).Minutes()) )
	}

}
