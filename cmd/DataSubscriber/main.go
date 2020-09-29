package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/QzSG/lapis-uno/protobuf"
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

var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

	reading := &pb.Reading{}

	if err := proto.Unmarshal(msg.Payload(), reading); err != nil {
		log.Fatalln("Failed to parse sensor reading:", err)
	}

	fmt.Printf("Elapsed[ms]: %s\n", time.Since(time.Unix(0, reading.GetTimeStamp())))
}

func init() {
	//log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

func main() {

	// Signal stuff to handle graceful exits
	signalChan := make(chan os.Signal, 1)
	done := make(chan struct{})

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go handleSignals(signalChan, done)

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

	if token := client.Subscribe(topic, 0, nil); token.Wait() && token.Error() != nil {
		log.Error(token.Error())
		os.Exit(1)
	}

	// Signal stuff
	<-done
}
