package util

import (
	"fmt"
	"math/rand"
	"time"

	pb "github.com/QzSG/lapis-uno/protobuf"
)

// RandFloats returns a float64 slice containing number of floats you need.
func RandFloats(minVal float64, maxVal float64, count int) []float64 {
	rand.Seed(time.Now().UnixNano())
	floats := make([]float64, count)
	for i := range floats {
		floats[i] = minVal + rand.Float64()*(maxVal-minVal)
	}
	return floats
}

// RandReading returns a Reading protobuf message with randomly generated values
func RandReading() *pb.Reading {

	floats := RandFloats(-10.00, 10.00, 6)
	isStart := false
	pos := rand.Intn(3) + 1
	if rand.Intn(2) == 1 {
		isStart = true
	} else {
		isStart = false
	}
	return &pb.Reading{
		IsStartMove: isStart,
		ClientID:    fmt.Sprint(rand.Intn(3)),
		DancerNo:    int32(rand.Intn(3)),
		AccX:        floats[0],
		AccY:        floats[1],
		AccZ:        floats[2],
		GyroRoll:    floats[3],
		GyroPitch:   floats[4],
		GyroYaw:     floats[5],
		TimeStamp:   time.Now().UnixNano(),
	}
}
