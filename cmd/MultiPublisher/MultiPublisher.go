package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
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
	log.Info("NTP Offset:", clockOffset)
	log.Info("NTP Clock:", clock.Add(time.Since(clock)+clockOffset))
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

	var wg sync.WaitGroup
	wg.Add(150)
	time.Sleep(2 * time.Second)
	ticker := time.NewTicker(time.Duration(20 * time.Millisecond))
	defer ticker.Stop()

	complete := make(chan struct{})
	go func() {
		time.Sleep(1 * time.Second)
		complete <- struct{}{}
	}()

T:
	for {
		select {
		case <-complete:
			wg.Wait()
			break T
		case <-ticker.C:
			go publishReading(client1, clockOffset, ClientID1, &wg)
			go publishReading(client2, clockOffset, ClientID2, &wg)
			go publishReading(client3, clockOffset, ClientID3, &wg)
		}
	}
	<-done
	client1.Disconnect(10)
	client2.Disconnect(10)
	client3.Disconnect(10)
}

func publishReading(client mqtt.Client, clockOffset time.Duration, clientID string, wg *sync.WaitGroup) {

	defer wg.Done()
	topic := fmt.Sprintf("sensor/%s/data", clientID)
	var reading *pb.Reading
	reading = util.RandReading()
	reading.TimeStamp = clock.Add(time.Since(clock) + clockOffset).UnixNano()
	payload, err := proto.Marshal(reading)
	if err != nil {
		log.Fatalln("Failed to encode sensor reading:", err)
	}
	client.Publish(topic, 0, false, payload)

}
