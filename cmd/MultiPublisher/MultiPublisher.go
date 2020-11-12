package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	pb "github.com/QzSG/lapis-uno/protobuf"

	ntp "github.com/QzSG/lapis-uno/cmd/NTP"
	"github.com/QzSG/lapis-uno/cmd/internal/util"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
)

func handleSignals(sigs <-chan os.Signal, done chan<- struct{}) {
	sig := <-sigs
	log.WithFields(log.Fields{
		"signal": sig,
	}).Info("Signal Received")

	client1.Disconnect(100)
	client2.Disconnect(100)
	client3.Disconnect(100)
	done <- struct{}{}
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

var (
	clock   = time.Now()
	client1 mqtt.Client
	client2 mqtt.Client
	client3 mqtt.Client
)

func main() {
	// Signal stuff to handle graceful exits
	signalChan := make(chan os.Signal, 1)
	done := make(chan struct{})

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go handleSignals(signalChan, done)

	log.Info("Starting NTPClient to get offset")

	clockOffset, err := ntp.Offset()
	if err != nil {
		log.Error(err.Error())
	}
	log.Info("NTP Offset:", clockOffset)
	log.Info("NTP Clock:", clock.Add(time.Since(clock)+clockOffset))
	const ClientID1 = "1"
	const ClientID2 = "2"
	const ClientID3 = "3"
	const BrokerConfig = "ssl://mqtts.qz.sg:8883"

	log.Info("Connecting to " + BrokerConfig)

	tlsConfig := &tls.Config{
		//Go will dig out and use the System RootCA cert set if nothing is passed in
		ClientAuth: tls.NoClientCert, //we do not use client certs for auth
		ClientCAs:  nil,
	}

	opts := mqtt.NewClientOptions().AddBroker(BrokerConfig).SetClientID(ClientID1)
	opts.SetTLSConfig(tlsConfig)
	opts.SetUsername("bench")
	opts.SetPassword("bench")

	/*log.WithFields(log.Fields{
		"Topic": topic,
	}).Info("Client set to publish to topic")
	*/
	client1 = mqtt.NewClient(opts)
	if token := client1.Connect(); token.Wait() && token.Error() != nil {
		log.Panic(token.Error())
	} else {
		log.Info("Client 1 Connected to MQTT Broker over TLS")
	}

	opts.SetClientID(ClientID2)
	client2 = mqtt.NewClient(opts)
	if token := client2.Connect(); token.Wait() && token.Error() != nil {
		log.Panic(token.Error())
	} else {
		log.Info("Client 2 Connected to MQTT Broker over TLS")
	}

	opts.SetClientID(ClientID3)
	client3 = mqtt.NewClient(opts)
	if token := client3.Connect(); token.Wait() && token.Error() != nil {
		log.Panic(token.Error())
	} else {
		log.Info("Client 3 Connected to MQTT Broker over TLS")
	}

	var wg sync.WaitGroup
	var periodms = 50
	wg.Add(3000 / periodms)
	time.Sleep(2 * time.Second)
	ticker := time.NewTicker(time.Duration(time.Duration(periodms) * time.Millisecond))
	defer ticker.Stop()

	complete := make(chan struct{})
	go func() {
		time.Sleep(1 * time.Second)
		complete <- struct{}{}
	}()
	var start = false
	var tickCount = 0
T:
	for {
		select {
		case <-complete:
			wg.Wait()
			break T
		case <-ticker.C:
			tickCount++
			go publishReading(client1, clockOffset, ClientID1, &wg, start, 2)
			go publishReading(client2, clockOffset, ClientID2, &wg, start, -1)
			go publishReading(client3, clockOffset, ClientID3, &wg, start, -1)
			if tickCount%3 == 0 {
				start = !start
			}
		}
	}
	<-done
}

func publishReading(client mqtt.Client, clockOffset time.Duration, clientID string, wg *sync.WaitGroup, start bool, posChange int32) {
	defer wg.Done()

	topic := fmt.Sprintf("sensor/%s/data", clientID)
	var reading *pb.Reading
	reading = util.RandReading()
	reading.ClientID = clientID
	dNo, _ := strconv.Atoi(clientID)
	reading.DancerNo = int32(dNo)
	reading.PosChange = posChange + 3
	reading.IsStartMove = start
	reading.TimeStamp = clock.Add(time.Since(clock) + clockOffset).UnixNano()
	var flag int
	if reading.IsStartMove {
		flag = 1
	} else {
		flag = 0
	}
	log.Debug(flag, " ", reading.ClientID, " ", reading.TimeStamp)
	payload, err := proto.Marshal(reading)
	if err != nil {
		log.Fatalln("Failed to encode sensor reading:", err)
	}
	token := client.Publish(topic, 0, false, payload)

	token.Wait()

}
