package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	ntp "github.com/QzSG/lapis-uno/cmd/NTP"
	"github.com/QzSG/lapis-uno/cmd/internal/util"
	pb "github.com/QzSG/lapis-uno/protobuf"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
)

var (
	clock = time.Now()
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

	const ClientID = "lapis-client-test-1"
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
	topic := fmt.Sprintf("sensor/%s/data", ClientID)

	log.WithFields(log.Fields{
		"Topic": topic,
	}).Info("Client set to publish to topic")

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Panic(token.Error())
	} else {
		log.Info("Connected to MQTT Broker over TLS")
	}

	var reading *pb.Reading
	time.Sleep(2 * time.Second)
	ticker := time.NewTicker(time.Duration(20 * time.Millisecond))
	defer ticker.Stop()
	complete := make(chan struct{})
	sleep := 1 * time.Second
	go func() {
		time.Sleep(sleep)
		complete <- struct{}{}
	}()

T:
	for {
		select {
		case <-complete:
			break T
		case <-ticker.C:

			reading = util.RandReading()
			readingTime := clock.Add(time.Since(clock) + clockOffset)
			reading.TimeStamp = readingTime.UnixNano()
			payload, err := proto.Marshal(reading)
			if err != nil {
				log.Fatalln("Failed to encode sensor reading:", err)
			}
			token := client.Publish(topic, 0, false, payload)
			token.Wait()
			log.WithFields(log.Fields{"Reading": reading}).Debug(reading)

		}
	}

	/*
					for i := 0; i < 20; i++ {
					reading = util.RandReading()

					payload, err := proto.Marshal(reading)
					if err != nil {
						log.Fatalln("Failed to encode sensor reading:", err)
					}
					token := client.Publish(topic, 0, false, payload)
					token.Wait()
					log.WithFields(log.Fields{"Reading": reading}).Debug(reading)
				}

		for i := 0; i < 20; i++ {
			text := make([]byte, 8)
			timenow := time.Now().UnixNano()
			binary.LittleEndian.PutUint64(text, uint64(timenow))
			token := client.Publish(topic, 0, false, text)
			token.Wait()
			fmt.Println(timenow)
		}
	*/

	// Signal stuff
	<-done
}
