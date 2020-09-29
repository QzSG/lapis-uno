package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

/* https://tools.ietf.org/html/rfc5905

    NTP Short Format
	   0                   1                   2                   3
       0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |          Seconds              |           Fraction            |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

    NTP Timestamp format
	  0                   1                   2                   3
       0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                            Seconds                            |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                            Fraction                           |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	NTP Packet

       0                   1                   2                   3
       0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |LI | VN  |Mode |    Stratum     |     Poll      |  Precision   |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                         Root Delay                            |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                         Root Dispersion                       |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                          Reference ID                         |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                                                               |
      +                     Reference Timestamp (64)                  +
      |                                                               |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                                                               |
      +                      Origin Timestamp (64)                    +
      |                                                               |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                                                               |
      +                      Receive Timestamp (64)                   +
      |                                                               |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                                                               |
      +                      Transmit Timestamp (64)                  +
      |                                                               |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                                                               |
      .                                                               .
      .                    Extension Field 1 (variable)               .
      .                                                               .
      |                                                               |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                                                               |
      .                                                               .
      .                    Extension Field 2 (variable)               .
      .                                                               .
      |                                                               |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                          Key Identifier                       |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
      |                                                               |
      |                            dgst (128)                         |
      |                                                               |
      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

                      Figure 8: Packet Header Format

*/

// NTPPacket : NTP packet v4 format
type NTPPacket struct {
	LiVnMode           uint8 // LI, VN, Mode (2|3|3) Client Mode must be 3
	Stratum            uint8 // stratum
	Poll               int8  // poll
	Precision          int8  // precision
	RootDelay          uint32
	RootDispersion     uint32
	ReferenceID        uint32 // identifier
	RefTimeSecond      uint32 // last time local clock was updated second
	RefTimeFraction    uint32 // last time local clock was updated fraction
	OriginTimeSecond   uint32 // client time second
	OriginTimeFraction uint32 // client time fraction
	RecvTimeSecond     uint32 // receive time second
	RecvTimeFraction   uint32 // receive time fraction
	TransTimeSec       uint32 // transmit time second
	TransTimeFrac      uint32 // transmit time fraction
}

// LiVNMode :
const LiVNMode uint8 = 0b11100011 // unknown li, v4, client mode
const server = "sg.pool.ntp.org:123"

var nanoPerSec = uint64(time.Second.Nanoseconds())
var ntpEpoch = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)

func main() {

	var transmitTime time.Time

	req := NTPPacket{
		LiVnMode: LiVNMode,
	}
	addr, _ := net.ResolveUDPAddr("udp", server)
	conn, _ := net.DialUDP("udp", nil, addr)
	defer conn.Close()

	fmt.Println(addr)
	transmitTime = time.Now()
	if err := binary.Write(conn, binary.BigEndian, req); err != nil {
		log.Fatalf("Failed to send NTP request: %v", err)
	}

	response := &NTPPacket{}
	if err := binary.Read(conn, binary.BigEndian, response); err != nil {
		log.Fatalf("Failed to read server response: %v", err)
	}

	delta := time.Since(transmitTime)
	recvTime := ntpEpoch.Add(durationFromNTP(uint32(ntpTime(transmitTime.Add(delta))>>32), uint32(ntpTime(transmitTime.Add(delta))&0xffffffff)))
	destRecvTime := ntpEpoch.Add(durationFromNTP(response.RecvTimeSecond, response.RecvTimeFraction))
	destTransTime := ntpEpoch.Add(durationFromNTP(response.TransTimeSec, response.TransTimeFrac))
	originTime := ntpEpoch.Add(durationFromNTP(uint32(ntpTime(transmitTime)>>32), uint32(ntpTime(transmitTime)&0xffffffff)))

	fmt.Println(originTime, destRecvTime, destTransTime, recvTime)

	forwardPath := destRecvTime.Sub(originTime)
	backPath := destTransTime.Sub(recvTime)
	offset := (forwardPath + backPath) / time.Duration(2)

	fmt.Println(forwardPath, backPath, offset)
}

func ntpTime(t time.Time) uint64 {
	nanosec := uint64(t.Sub(ntpEpoch))
	sec := nanosec / nanoPerSec
	nanosec = uint64(nanosec-sec*nanoPerSec) << 32
	fraction := uint64(nanosec / nanoPerSec)
	if nanosec%nanoPerSec >= nanoPerSec/2 {
		fraction++
	}
	return uint64(sec<<32 | fraction)

}

func durationFromNTP(sec uint32, frac uint32) time.Duration {
	seconds := uint64(sec) * nanoPerSec
	fraction := uint64(frac) * nanoPerSec
	nanosec := fraction >> 32
	return time.Duration(seconds + nanosec)
}
