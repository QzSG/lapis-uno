package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	ntp "github.com/QzSG/lapis-uno/cmd/NTP"
	pb "github.com/QzSG/lapis-uno/protobuf"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
)

var (
	clock    = time.Now()
	offset   time.Duration
	start    = make(chan startPacket)
	calcDone = make(chan struct{})
)

func handleSignals(sigs <-chan os.Signal, done chan<- struct{}) {
	sig := <-sigs
	log.WithFields(log.Fields{
		"signal": sig,
	}).Info("Signal Received")
	calcDone <- struct{}{}
	done <- struct{}{}
}

var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

	clientID := strings.Split(msg.Topic(), "/")[1]
	fmt.Println("Message from:", clientID)
	reading := &pb.Reading{}

	if err := proto.Unmarshal(msg.Payload(), reading); err != nil {
		log.Fatalln("Failed to parse sensor reading:", err)
	}
	if reading.IsStartMove {
		log.Info(time.Unix(0, reading.GetTimeStamp()).UnixNano(), " ", clientID)
		start <- startPacket{clientID: clientID, timeStamp: reading.GetTimeStamp()}
	}
	fmt.Printf("Elapsed[ms]: %s\n", clock.Add(time.Since(clock)+offset).Sub(time.Unix(0, reading.GetTimeStamp())))
}

func init() {
	//log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

type startPacket struct {
	clientID  string
	timeStamp int64
}

func calcLatency() {
	var Client1, Client2, Client3 startPacket
	var syncDelay time.Duration
	received := 0
	for {
		select {
		case pack := <-start:
			received++
			log.Info("Received start packet from", pack.clientID)
			if received == 1 {
				Client1 = pack
			}
			if received == 2 {
				Client2 = pack
			}
			if received == 3 {
				Client3 = pack
				packets := []startPacket{Client1, Client2, Client3}
				sort.Slice(packets, func(i, j int) bool { return packets[i].timeStamp < packets[j].timeStamp })
				log.Info("Calculating syncDelay...")
				syncDelay = time.Unix(0, packets[2].timeStamp).Sub(time.Unix(0, packets[0].timeStamp))
				fmt.Println("syncDelay between ", packets[0].clientID, " and ", packets[2].clientID, " is :", syncDelay)
				received = 0
			}
		case <-calcDone:
			return
		}
	}
}

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
	offset = clockOffset

	log.Info("Clock Offset:", offset)

	const ClientID = "lapis-client-test-0"
	const BrokerConfig = "ssl://mqtts.qz.sg:8883"

	log.Info("Connecting to " + BrokerConfig)

	tlsConfig := &tls.Config{
		//Go will dig out and use the System RootCA cert set if nothing is passed in
		ClientAuth: tls.NoClientCert, //we do not use client certs for auth
		ClientCAs:  nil,
	}

	opts := mqtt.NewClientOptions().AddBroker(BrokerConfig).SetClientID(ClientID)
	opts.SetTLSConfig(tlsConfig)
	opts.SetUsername("bench")
	opts.SetPassword("bench")
	opts.SetDefaultPublishHandler(f)
	topic := "sensor/+/data"

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Panic(token.Error())
	} else {
		log.Infoln("Connected to MQTT Broker over TLS")
	}

	go calcLatency()
	if token := client.Subscribe(topic, 0, nil); token.Wait() && token.Error() != nil {
		log.Error(token.Error())
		os.Exit(1)
	}

	// Signal stuff
	<-done
	client.Disconnect(10)
}
