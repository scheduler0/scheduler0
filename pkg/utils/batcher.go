package utils

import (
	"encoding/json"
	"log"
	"math"
	"scheduler0/pkg/constants"
)

// Batch returns a slice containing slices with a
// size that's optimized to not trigger a sqlite "too much variable" error
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

// BatchByBytes returns a slice of bytes in which each byte is less than the max batch size
func BatchByBytes[T any](data []T, maxBatchSizeMb int) [][]byte {
	collection := []T{data[0]}
	collectionSmall, err := json.Marshal(collection)
	if err != nil {
		log.Fatal("failed to convert json to byte", err)
	}

	unitSize := float64(len(collectionSmall)) / 1000000
	itemsPerBatch := int(math.Floor((float64(maxBatchSizeMb) - (unitSize * 10)) / unitSize))
	var results = [][]byte{}

	i := 0

	currentCollection := []T{}
	for i < len(data) {

		if (i%itemsPerBatch) == 0 && i > 0 {
			currentCollectionBytes, err := json.Marshal(currentCollection)
			if err != nil {
				log.Fatal("failed to convert json to byte", err)
			}
			results = append(results, currentCollectionBytes)
			currentCollection = []T{}
		}

		currentCollection = append(currentCollection, data[i])

		i++
	}

	if len(currentCollection) > 0 {
		currentCollectionBytes, err := json.Marshal(currentCollection)
		if err != nil {
			log.Fatal("failed to convert json to byte", err)
		}
		results = append(results, currentCollectionBytes)
		currentCollection = []T{}
	}

	return results
}
