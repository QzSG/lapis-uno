package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
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
	done <- struct{}{}
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

var (
	clock = time.Now()
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
	log.Info("Clock Offset:", clockOffset)

	const ClientID1 = "lapis-client-pub-1"
	const ClientID2 = "lapis-client-pub-2"
	const ClientID3 = "lapis-client-pub-3"
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
	client1 := mqtt.NewClient(opts)
	if token := client1.Connect(); token.Wait() && token.Error() != nil {
		log.Panic(token.Error())
	} else {
		log.Info("Client 1 Connected to MQTT Broker over TLS")
	}

	opts.SetClientID(ClientID2)
	client2 := mqtt.NewClient(opts)
	if token := client2.Connect(); token.Wait() && token.Error() != nil {
		log.Panic(token.Error())
	} else {
		log.Info("Client 2 Connected to MQTT Broker over TLS")
	}

	opts.SetClientID(ClientID3)
	client3 := mqtt.NewClient(opts)
	if token := client3.Connect(); token.Wait() && token.Error() != nil {
		log.Panic(token.Error())
	} else {
		log.Info("Client 3 Connected to MQTT Broker over TLS")
	}
	var last int
T:
	for {

		start := publishReading(client1, clockOffset, ClientID1)
		_, err = fmt.Scan(&last)

		if last == 2 {
			fmt.Println("Elapsed |", time.Since(start)+clockOffset, "Sending packet for client 3")
			mid := publishReading(client3, clockOffset, ClientID3)

			fmt.Println("Elapsed |", mid.Sub(start), "Sending packet for client 2")
			end := publishReading(client2, clockOffset, ClientID2)

			fmt.Println("Elapsed |", end.Sub(start), "Sync Delay between Client 1 and 2")
		}
		if last == 3 {
			fmt.Println("Elapsed |", time.Since(start)+clockOffset, "Sending packet for client 2")
			mid := publishReading(client2, clockOffset, ClientID2)

			fmt.Println("Elapsed |", mid.Sub(start), "Sending packet for client 3")
			end := publishReading(client3, clockOffset, ClientID3)

			fmt.Println("Elapsed |", end.Sub(start), "Sync Delay between Client 1 and 3")
		}
		if last == 4 {
			break T
		}
	}
	<-done
	client1.Disconnect(10)
	client2.Disconnect(10)
	client3.Disconnect(10)
}

func publishReading(client mqtt.Client, clockOffset time.Duration, clientID string) time.Time {

	topic := fmt.Sprintf("sensor/%s/data", clientID)
	var reading *pb.Reading
	reading = util.RandReading()
	reading.IsStartMove = true
	readingTime := clock.Add(time.Since(clock) + clockOffset)
	reading.TimeStamp = readingTime.UnixNano()
	payload, err := proto.Marshal(reading)
	if err != nil {
		log.Fatalln("Failed to encode sensor reading:", err)
	}
	token := client.Publish(topic, 0, false, payload)
	token.Wait()
	return readingTime
}
