package utils

import (
	"math"
	"scheduler0/constants"
	"scheduler0/repository/fixtures"
	"testing"
)

func TestBatchByBytes(t *testing.T) {
	t.Run("should return slice with batches", func(t *testing.T) {
		var numOfJobs int = 100000
		numberOfColumns := 4
		jobFixture := fixtures.JobFixture{}
		jobs := jobFixture.CreateNJobModels(numOfJobs)

		maxVar := int(math.Floor(float64(constants.DBMaxVariableSize / numberOfColumns)))

		batches := Batch(jobs, int64(numberOfColumns))

		for _, batch := range batches {
			if len(batch) > maxVar {
				t.Fatal("batch size should be less than ", maxVar, "but instead it is ", len(batch))
			}
		}
	})

	t.Run("should create batches of mb size", func(t *testing.T) {
		var numOfJobs int = 100000
		jobFixture := fixtures.JobFixture{}
		jobs := jobFixture.CreateNJobModels(numOfJobs)

		batches := BatchByBytes(jobs, 2)

		for _, batch := range batches {

			size := math.Ceil(float64(len(batch)/1000000)) / 2

			if size >= 1 {
				t.Fatal("batch size should be less than 2 mb")
			}
		}
	})

}
