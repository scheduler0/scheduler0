package utils

import (
	"math"
	"scheduler0/constants"
)

func Batch[T any](ids []T, numberOfColumns int64) [][]T {
	maxVar := int64(math.Floor(float64(constants.DBMaxVariableSize / numberOfColumns)))

	batches := [][]T{}

	if int64(len(ids)) > maxVar {
		temp := []T{}
		count := 0

		for count < len(ids) {
			temp = append(temp, ids[count])
			if int64(len(temp)) == maxVar {
				batches = append(batches, temp)
				temp = []T{}
			}
			count += 1
		}

		if len(temp) > 0 {
			batches = append(batches, temp)
			temp = []T{}
		}
	} else {
		batches = append(batches, ids)
	}

	return batches
}
