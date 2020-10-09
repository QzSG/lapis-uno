package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	ntp "github.com/QzSG/lapis-uno/cmd/NTP"
	pb "github.com/QzSG/lapis-uno/protobuf"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var (
	clock      = time.Now()
	port       = 10101
	offset     time.Duration
	grpcServer *grpc.Server
)

func handleSignals(sigs <-chan os.Signal, done chan<- struct{}) {
	sig := <-sigs
	log.WithFields(log.Fields{
		"signal": sig,
	}).Info("Signal Received")
	grpcServer.GracefulStop()
	done <- struct{}{}
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

type sensorServer struct {
	pb.UnimplementedSensorServer
	mqttClient mqtt.Client
	topic      string
}

func (s *sensorServer) ReadingStream(stream pb.Sensor_ReadingStreamServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		reading := in

		log.Debug("Reading : ", reading)
		reading.TimeStamp = clock.Add(time.Since(clock) + offset).UnixNano()
		payload, err := proto.Marshal(reading)
		if err != nil {
			log.Fatalln("Failed to encode sensor reading:", err)
		}
		token := s.mqttClient.Publish(s.topic, 0, false, payload)
		token.Wait()

		if err := stream.Send(&pb.Reply{Status: 1}); err != nil {
			return err
		}
	}
}

func newServer() *sensorServer {
	const ClientID = "lapis-client-test-1"
	const BrokerConfig = "ssl://mqtts.qz.sg:8883"

	log.Info("Connecting to " + BrokerConfig)

	tlsConfig := &tls.Config{
		//Go will dig out and use the System RootCA cert set if nothing is passed in
		ClientAuth: tls.NoClientCert, //we do not use client certs for auth
		ClientCAs:  nil,
	}

	mqttOpts := mqtt.NewClientOptions().AddBroker(BrokerConfig).SetClientID(ClientID)
	mqttOpts.SetTLSConfig(tlsConfig)
	mqttOpts.SetUsername("bench")
	mqttOpts.SetPassword("bench")
	topic := fmt.Sprintf("sensor/%s/data", ClientID)

	log.WithFields(log.Fields{
		"Topic": topic,
	}).Info("Client set to publish to topic")

	client := mqtt.NewClient(mqttOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Panic(token.Error())
	} else {
		log.Info("Connected to MQTT Broker over TLS")
	}
	s := &sensorServer{
		mqttClient: client,
		topic:      topic,
	}
	return s
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

	offset = clockOffset
	log.Info("NTP Clock:", clock.Add(time.Since(clock)+clockOffset))

	log.Info("Starting GRPC Server on port", port)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer = grpc.NewServer(opts...)
	pb.RegisterSensorServer(grpcServer, newServer())
	grpcServer.Serve(listener)
	<-done
}
