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

		println("Diff seconds", endDate.Sub(startDate).Minutes())
	}

}
