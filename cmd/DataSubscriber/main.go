package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	ntp "github.com/QzSG/lapis-uno/cmd/NTP"
	pb "github.com/QzSG/lapis-uno/protobuf"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
)

var (
	clock               = time.Now()
	offset              time.Duration
	file                *os.File //for single mode
	file1, file2, file3 *os.File //for multi mode
	c1, c2, c3          string   //for multi mode

	cid            string
	mode           string
	evalClientConn string
	ignore         string

	start    = make(chan startPacket)
	calcDone = make(chan struct{})

	msgChan = make(chan message, 10) //Buffered channel for posting to evalclient using httppost

	zerostartPacket = &startPacket{}

	startclient1, startclient2, startclient3 startPacket //used only for multi mode
	idleclient1, idleclient2, idleclient3    startPacket //used only for multi mode
	startCount                               = 0
	idleCount                                = 0
	waitForNextMove                          = false
	hasAllFirstStartPackets                  = false
	hasAllFirstIdlePackets                   = false

	initClients = false
)

// Generic message struct
type message struct {
	msgType   string
	data      string
	ts        string
	cids      string
	extraData string
}

type startPacket struct {
	clientID  string
	timeStamp int64
	dancerNo  int32
	posChange int32
}

func (s *startPacket) Reset() {
	*s = *zerostartPacket
}

func handleSignals(sigs <-chan os.Signal, done chan<- struct{}) {
	sig := <-sigs
	log.WithFields(log.Fields{
		"signal": sig,
	}).Info("Signal Received")

	if mode != "single" {
		calcDone <- struct{}{}
	}

	done <- struct{}{}
}

var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

	reading := &pb.Reading{}

	if err := proto.Unmarshal(msg.Payload(), reading); err != nil {
		log.Fatalln("Failed to parse sensor reading:", err)
	}

	//fmt.Printf("Elapsed[ms]: %s\n", clock.Add(time.Since(clock)+offset).Sub(time.Unix(0, reading.GetTimeStamp())))
	//fmt.Println("Reading:", reading)
	out := fmt.Sprint(reading.IsStartMove, " ", reading.ClientID, " ", reading.DancerNo,
		reading.AccX, reading.AccY, reading.AccZ,
		reading.GyroRoll, reading.GyroPitch, reading.GyroYaw, reading.TimeStamp, "\n")

	//Multi mode
	if mode != "single" {

		// If waiting for next move to start and a next start packet arrives from any client, reset bools
		if waitForNextMove && reading.IsStartMove {
			hasAllFirstStartPackets = false
			hasAllFirstIdlePackets = false
			waitForNextMove = false
		}

		// If all booleans reset && first move from any client, start calculation loop
		if !hasAllFirstStartPackets && !waitForNextMove && reading.IsStartMove {
			//log.Info("New move")
			switch startCount {
			case 0:
				startclient1 = startPacket{clientID: reading.GetClientID(), timeStamp: reading.GetTimeStamp(), dancerNo: reading.GetDancerNo(), posChange: reading.GetPosChange()}
				start <- startclient1
				startCount++
				log.Debug("First start packet from ", startclient1.clientID)
			case 1:
				if reading.ClientID != startclient1.clientID {
					startclient2 = startPacket{clientID: reading.GetClientID(), timeStamp: reading.GetTimeStamp(), dancerNo: reading.GetDancerNo(), posChange: reading.GetPosChange()}
					start <- startclient2
					startCount++
					log.Debug("Second start packet from ", startclient2.clientID)
				}
			case 2:
				if (reading.ClientID != startclient1.clientID) && (reading.ClientID != startclient2.clientID) {
					startclient3 = startPacket{clientID: reading.GetClientID(), timeStamp: reading.GetTimeStamp(), dancerNo: reading.GetDancerNo(), posChange: reading.GetPosChange()}
					start <- startclient3
					startCount++
					hasAllFirstStartPackets = true
					log.Debug("Last start packet from ", startclient3.clientID)
					log.Debug("Received first start packets for all 3 clients")
					log.Debug("1: ", startclient1.clientID, " 2 :", startclient2.clientID, " 3: ", startclient3.clientID)
					startclient1.Reset()
					startclient2.Reset()
					startclient3.Reset()
					startCount = 0
				}
			}
		}
		// Assumes all start packets will arrive long before first idle packet arrives
		// If first idle packet not recv from each client, if all first start packets recved, if its not a start move, loop and wait for all idle packets (one from each client)
		if !hasAllFirstIdlePackets && hasAllFirstStartPackets && !reading.IsStartMove {
			//log.Debug("Entered idle")
			switch idleCount {
			case 0:
				idleclient1 = startPacket{clientID: reading.GetClientID(), timeStamp: reading.GetTimeStamp(), dancerNo: reading.GetDancerNo()}
				idleCount++
				log.Debug("First idle packet from ", idleclient1.clientID)
			case 1:
				if reading.ClientID != idleclient1.clientID {
					idleclient2 = startPacket{clientID: reading.GetClientID(), timeStamp: reading.GetTimeStamp(), dancerNo: reading.GetDancerNo()}
					idleCount++
					log.Debug("Second idle packet from ", idleclient2.clientID)
				}
			case 2:
				if (reading.ClientID != idleclient1.clientID) && (reading.ClientID != idleclient2.clientID) {
					idleclient3 = startPacket{clientID: reading.GetClientID(), timeStamp: reading.GetTimeStamp(), dancerNo: reading.GetDancerNo()}
					idleCount++
					hasAllFirstIdlePackets = true
					log.Debug("Last idle packet from ", idleclient3.clientID)
					log.Debug("Received first idle packets for all 3 clients, waiting for next start packet loop")
					idleclient1.Reset()
					idleclient2.Reset()
					idleclient3.Reset()
					idleCount = 0
				}
			}
		}
		// Has all first start and first idle packets, waiting for next move to start
		if hasAllFirstStartPackets && hasAllFirstIdlePackets {
			log.Debug("Waiting for next move to start")
			waitForNextMove = true
		}

		switch reading.ClientID {
		case "1":
			file1.WriteString(out)
			file1.Sync()

		case "2":
			file2.WriteString(out)
			file2.Sync()
		case "3":
			file3.WriteString(out)
			file3.Sync()
		}
	}
	file.WriteString(out)
	file.Sync()

}

func calcRoutine() {
	var Client1, Client2, Client3 startPacket
	var syncDelay time.Duration
	received := 0
	for {
		select {
		case pack := <-start:
			received++
			log.Info("Received start packet from ", pack.clientID)
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
				log.Info(packets)
				log.WithFields(log.Fields{
					"Fastest":   packets[0].clientID,
					"Slowest":   packets[2].clientID,
					"SyncDelay": syncDelay,
				}).Info("SyncDelay calculated")
				msgChan <- message{msgType: "delay", data: fmt.Sprint(syncDelay.Seconds() * 1000.00), ts: fmt.Sprint(clock.Add(time.Since(clock) + offset).UnixNano())}

				if ignore != "pos" {
					log.Info("Scaling down posChanges values")
					log.Info("Current |", packets[0].posChange, packets[1].posChange, packets[2].posChange)

					packets[0].posChange = packets[0].posChange - 3
					packets[1].posChange = packets[1].posChange - 3
					packets[2].posChange = packets[2].posChange - 3

					log.Info("Scaled |", packets[0].posChange, packets[1].posChange, packets[2].posChange)

					msgChan <- message{
						msgType:   "positions",
						data:      fmt.Sprint(packets[0].dancerNo, packets[1].dancerNo, packets[2].dancerNo),    // one single string for all initial pos ie: 0 2 3
						extraData: fmt.Sprint(packets[0].posChange, packets[1].posChange, packets[2].posChange), // one single string for all posChange ie: -1 0 1
						cids:      fmt.Sprint(packets[0].clientID, " ", packets[1].clientID, " ", packets[2].clientID),
						ts:        fmt.Sprint(clock.Add(time.Since(clock) + offset).UnixNano()),
					}
				}
				received = 0
			}
		case <-calcDone:
			return
		}
	}
}

func postData() {
	defer close(msgChan)
	for {
		select {
		case msg := <-msgChan:
			var reqBody []byte
			var err error
			if msg.msgType == "positions" {
				reqBody, err = json.Marshal(map[string]string{
					"dancerNo": msg.data,
					"changes":  msg.extraData,
					"cids":     msg.cids,
					"ts":       msg.ts,
				})
			} else {
				reqBody, err = json.Marshal(map[string]string{
					msg.msgType: msg.data,
					"ts":        msg.ts,
				})
			}

			if err != nil {
				log.Error(err)
			} else {
				url := evalClientConn + "/" + msg.msgType
				resp, err := http.Post(url,
					"application/json", bytes.NewBuffer(reqBody))
				if err != nil {
					log.Error(err)
					log.Error("Could not post to EvalClient on ", evalClientConn, " but continuing silently")
				} else {
					defer resp.Body.Close()
					respBody, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						log.Error(err)
					} else {
						log.Info("Response body | ", string(respBody))
					}
				}
			}
		}
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
	flag.StringVar(&cid, "cid", "lapis-client-sub-"+fmt.Sprint(rand.Intn(1000)), "If not provided, defaults to lapis-client-sub-X where X is a random int between 1 & 1000")
	flag.StringVar(&mode, "mode", "single", "Enter mode: single or multi, defaults to single. Single mode will not perform position nor latency calculation")
	flag.StringVar(&evalClientConn, "evalclientconn", "http://127.0.0.1:10202", "please enter http://<ip>:<port> of evalclient httpserver, for example: -evalclientconn=http://127.0.0.1:10202")
	flag.StringVar(&ignore, "ignore", "none", "Enter ignore: pos, defaults to none. pos will not calculate positions")
	//log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

func main() {

	// Signal stuff to handle graceful exits
	signalChan := make(chan os.Signal, 1)
	done := make(chan struct{})

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go handleSignals(signalChan, done)

	flag.Parse()
	log.Info("Starting in " + mode + " mode")
	log.Info("Ignoring | " + ignore)
	log.Info("Starting NTPClient to get offset")

	clockOffset, err := ntp.Offset()
	if err != nil {
		log.Error(err.Error())
	}
	offset = clockOffset

	log.Info("NTP Offset:", offset)
	log.Info("NTP Clock:", clock.Add(time.Since(clock)+offset))

	var ClientID = cid
	const BrokerConfig = "ssl://mqtts.qz.sg:8883"

	if mode != "single" {
		go calcRoutine()
		go postData()
	}

	file, err = os.OpenFile("reading.csv", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()
	if mode != "single" {
		file1, err = os.OpenFile("reading_1.csv", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			log.Panic(err)
		}
		defer file1.Close()
		file2, err = os.OpenFile("reading_2.csv", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			log.Panic(err)
		}
		defer file2.Close()
		file3, err = os.OpenFile("reading_3.csv", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			log.Panic(err)
		}
		defer file3.Close()
	}

	log.Info("Connecting to " + BrokerConfig + " with ClientID " + ClientID)

	tlsConfig := &tls.Config{
		//Go will dig out and use the System RootCA cert set if nothing is passed in
		ClientAuth: tls.NoClientCert, //we do not use client certs for auth
		ClientCAs:  nil,
	}

	opts := mqtt.NewClientOptions().AddBroker(BrokerConfig).SetClientID(ClientID)
	opts.SetTLSConfig(tlsConfig)
	opts.SetUsername("xilinx")
	opts.SetPassword("undecimus")
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
