package util

import (
	"fmt"
	"math/rand"
	"time"

	pb "github.com/QzSG/lapis-uno/protobuf"
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

// RandReading returns a Reading protobuf message with randomly generated values
func RandReading() *pb.Reading {

	floats := RandFloats(-10.00, 10.00, 6)
	isStart := false

	if rand.Intn(2) == 1 {
		isStart = true
	} else {
		isStart = false
	}
	return &pb.Reading{
		IsStartMove: isStart,
		ClientID:    fmt.Sprint(rand.Intn(3)),
		AccX:        floats[0],
		AccY:        floats[1],
		AccZ:        floats[2],
		GyroX:       floats[3],
		GyroY:       floats[4],
		GyroZ:       floats[5],
		TimeStamp:   time.Now().UnixNano(),
	}
}
