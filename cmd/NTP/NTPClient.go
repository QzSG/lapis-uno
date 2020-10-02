package ntp

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
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

const liVNMode uint8 = 0b11100011    // unknown li, v4, client mode
const server = "sg.pool.ntp.org:123" // using sg ntp pool as default server source alt: time.google.com

var nanoPerSec = uint64(time.Second.Nanoseconds())
var ntpEpoch = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC) //ntp epoch starts at 1900

// Offset : Returns clock offset of system clock vs reference clock using NTP. Offset is returned as time.Duration
func Offset() (time.Duration, error) {
	var transmitTime time.Time

	req := NTPPacket{
		LiVnMode: liVNMode,
	}
	addr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		return time.Duration(0), err
	}
	conn, _ := net.DialUDP("udp", nil, addr)
	if err != nil {
		return time.Duration(0), err
	}
	defer conn.Close()

	//fmt.Println(addr)
	transmitTime = time.Now()
	//fmt.Println(transmitTime.String())
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

	//fmt.Println(originTime.String(), destRecvTime.String(), destTransTime.String(), recvTime.String())

	forwardPath := destRecvTime.Sub(originTime)
	backPath := destTransTime.Sub(recvTime)
	offset := (forwardPath + backPath) / time.Duration(2)

	//fmt.Println(forwardPath, backPath, offset)
	return offset, nil
}

func main() {
	clockOffset, err := Offset()
	if err != nil {
		log.Error(err.Error())
	}

	clock := time.Now()
	fmt.Printf("Clock offset : %s \n", clockOffset)
	fmt.Println(time.Now().Add(clockOffset))

	ticker := time.NewTicker(time.Second)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Println(clock.Add(time.Since(clock) + clockOffset))
			}
		}
	}()

	time.Sleep(10000 * time.Second)
	ticker.Stop()
	done <- true
	fmt.Println("Ticker stopped")
}

func ntpTime(unixTime time.Time) uint64 {
	nanosec := uint64(unixTime.Sub(ntpEpoch))
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
