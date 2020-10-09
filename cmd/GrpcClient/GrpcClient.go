package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/QzSG/lapis-uno/cmd/internal/util"
	pb "github.com/QzSG/lapis-uno/protobuf"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func handleSignals(sigs <-chan os.Signal, done chan<- struct{}) {
	sig := <-sigs
	log.WithFields(log.Fields{
		"signal": sig,
	}).Info("Signal Received")
	done <- struct{}{}
}

func init() {
	//log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

func runReadingStream(client pb.SensorClient) {
	stream, err := client.ReadingStream(context.Background())
	if err != nil {
		log.Fatal(client, err)
	}
	waitServerClose := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				close(waitServerClose)
				return
			}
			if err != nil {
				log.Fatalf("Failed to receive a reply status : %v", err)
			}
			log.Debug("Got reply status ", in.Status)
		}
	}()

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
			if err := stream.Send(reading); err != nil {
				log.Fatalf("Failed to send a reading: %v", err)
			}
			log.Debug(reading)

		}
	}

	stream.CloseSend()
	<-waitServerClose
}

func main() {

	// Signal stuff to handle graceful exits
	signalChan := make(chan os.Signal, 1)
	done := make(chan struct{})

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go handleSignals(signalChan, done)

	var serverAddr = "127.0.0.1:10101"
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())
	opts = append(opts, grpc.WithKeepaliveParams(
		keepalive.ClientParameters{
			Time:                10 * time.Second,
			PermitWithoutStream: true,
		}))
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, serverAddr, opts...)
	if err != nil {
		log.Fatalf("Error while dialing. Err: %v", err)
	}
	defer conn.Close()
	client := pb.NewSensorClient(conn)

	runReadingStream(client)

	// Signal stuff
	<-done
}
