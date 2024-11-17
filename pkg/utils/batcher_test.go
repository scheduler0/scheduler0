package utils

import (
	"github.com/brianvoe/gofakeit/v6"
	"math"
	"scheduler0/pkg/constants"
	"scheduler0/pkg/models"
	"testing"
)

func TestBatchByBytes(t *testing.T) {

	t.Run("should return slice with batches", func(t *testing.T) {
		var numOfJobs int = 100000
		numberOfColumns := 4
		jobs := []models.Job{}
		for i := 0; i < numOfJobs; i++ {
			var jobModel models.Job
			gofakeit.Struct(&jobModel)
			jobs = append(jobs, jobModel)
		}

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
		jobs := []models.Job{}
		for i := 0; i < numOfJobs; i++ {
			var jobModel models.Job
			gofakeit.Struct(&jobModel)
			jobs = append(jobs, jobModel)
		}

		batches := BatchByBytes(jobs, 2)

		for _, batch := range batches {
			size := math.Ceil(float64(len(batch)/1000000)) / 2
			if size > 1 {
				t.Fatal("batch size should be less than 2 mb", size)
			}
		}
	})

}
