package util

import (
	"math/rand"
	"time"
)

// RandFloats returns a float32 slice containing number of floats you need.
func RandFloats(minVal float32, maxVal float32, count int) []float32 {
	rand.Seed(time.Now().UnixNano())
	floats := make([]float32, count)
	for i := range floats {
		floats[i] = minVal + rand.Float32()*(maxVal-minVal)
	}
	return floats
}
