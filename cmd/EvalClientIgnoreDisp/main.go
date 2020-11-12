package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	log "github.com/sirupsen/logrus"
)

const key = "testtesttesttest"

var (
	dataChannel      = make(chan []byte)
	connectionString string
	dashConnString   string
	mode             string

	posChan   = make(chan posBody)
	moveChan  = make(chan moveBody)
	delayChan = make(chan string)

	recvDelay = "0"

	correctPos string              // correct positions returns by eval server, read only
	calcPos    = "1 2 3"           // calculated positions
	places     = make(map[int]int) //places[1] returns dancer number in left move pos, currently unused

	dancerNoToPlace = make(map[int]int) //tracks dancerno to place
)

type moveBody struct {
	Move string
	Ts   string
	Cid  string
}

type delayBody struct {
	Delay string
	Ts    string
}

type posBody struct {
	DancerNo string
	Changes  string
	Cids     string
	Ts       string
}

type posStruct struct {
	dancerNo int
	change   int
	cid      string
}

// Client : TCP Client
type Client struct {
	sock net.Conn
}

func (client *Client) recv() {
	for {
		msg := make([]byte, 8)
		len, err := client.sock.Read(msg)
		if err != nil {
			client.sock.Close()
			break
		}
		if len > 0 {
			fmt.Println("RECEIVED: " + string(msg))

			correctPos = strings.TrimRightFunc(string(msg), func(r rune) bool {
				return !unicode.IsPrint(r)
			})

			if calcPos != correctPos {
				log.Info("Calculated postiions were incorrect")

				positions := strings.Split(correctPos, " ")

				for i, dNoStr := range positions {
					dNo, _ := strconv.Atoi(dNoStr)
					dancerNoToPlace[dNo] = i + 1
				}
				calcPos = correctPos

				log.Info("Positions updated to |", calcPos)
			}
		}
	}
}

func (client *Client) send() {
	defer client.sock.Close()
	for {
		select {
		case message, ok := <-dataChannel:
			if !ok {
				return
			}
			(*client).sock.Write(message)
		}
	}
}

func encode(bytes []byte) string {
	return base64.StdEncoding.EncodeToString(bytes)
}

// Pad : Whitespace Padding
func Pad(srcBytes []byte, blockSize int) []byte {
	padding := blockSize - len(srcBytes)%blockSize
	padtext := bytes.Repeat([]byte{32}, padding)
	return append(srcBytes, padtext...)
}

func generateIV() []byte {
	iv := make([]byte, 16)

	if _, err := rand.Read(iv); err != nil {
		log.Error("Error occured generating IV")
	}

	return iv
}

// AESEncrypt : Encrypt data with AES-128-CBC
func AESEncrypt(data []byte) string {
	iv := generateIV()
	ciph, err := aes.NewCipher([]byte(key))
	if err != nil {
		log.Error("Error with the key: ", err)
	}
	encrypter := cipher.NewCBCEncrypter(ciph, iv)
	ciphertext := make([]byte, len(data))
	encrypter.CryptBlocks(ciphertext, data)

	ivData := append(iv, ciphertext...)
	return encode(ivData)
}

func clientStart() { //Client {
	log.Info("Lapis Comms Client Starting...")
	conn, err := net.Dial("tcp", connectionString)
	if err != nil {
		log.Fatal(err)
	} else {
		log.Info("Lapis Comms Client connected.")
	}
	client := &Client{sock: conn}
	go client.recv()
	go client.send()
}

// MoveCount :
type MoveCount struct {
	move  string
	count int
}

func updateRoutine() {
	var move1, move2, move3 string
	moveCount := 0
	allDancersIdentified := false

	recvMoves := make(map[string]string)

	m := make(map[string]int)
	for {
		select {
		case movebody := <-moveChan:
			moveCount++
			recvMoves[movebody.Cid] = movebody.Move
			switch moveCount {
			case 1:
				move1 = movebody.Move
			case 2:
				move2 = movebody.Move
			case 3:
				move3 = movebody.Move
				moves := []string{move1, move2, move3}
				for _, mv := range moves {
					if _, ok := m[mv]; ok {
						m[mv]++
					} else {
						m[mv] = 1
					}
				}

				moveCounts := make([]MoveCount, 0, len(m))
				for key, val := range m {
					moveCounts = append(moveCounts, MoveCount{move: key, count: val})
				}

				// sort wordCount slice by decreasing count number
				sort.Slice(moveCounts, func(i, j int) bool {
					return moveCounts[i].count > moveCounts[j].count
				})

				log.Info("Modal move | ", moveCounts[0].move, " | ", moveCounts[0].count)

				confMove := moveCounts[0].move
				data := "#" + calcPos + "|" + confMove + "|" + recvDelay
				log.Info("Sending | ", data)

				if mode != "standalone" {
					dataChannel <- []byte(AESEncrypt(Pad([]byte(data), aes.BlockSize)))
				}
				m = make(map[string]int)
				moveCount = 0

				go func(recvMoves map[string]string) {

					postBody := fmt.Sprint(data, "|", recvMoves["1"], " ", recvMoves["2"], " ", recvMoves["3"])
					reqBody, err := json.Marshal(map[string]string{
						"data": postBody,
					})
					log.Info("Posting | ", string(reqBody))
					if err != nil {
						log.Error(err)
					} else {
						resp, err := http.Post(dashConnString,
							"application/json", bytes.NewBuffer(reqBody))
						if err != nil {
							log.Error(err)
							log.Error("Could not post to dashconnection on ", dashConnString, " but continuing silently")
						} else {
							defer resp.Body.Close()
							respBody, err := ioutil.ReadAll(resp.Body)
							if err != nil {
								log.Error(err)
							} else {
								log.Info(string(respBody))
							}
						}
					}
				}(recvMoves)
			}
		case pos := <-posChan:

			places[1] = 0
			places[2] = 0
			places[3] = 0

			changes := strings.Split(pos.Changes, " ")
			s1, _ := strconv.Atoi(changes[0])
			s2, _ := strconv.Atoi(changes[1])
			s3, _ := strconv.Atoi(changes[2])

			clientids := strings.Split(pos.Cids, " ")
			log.Info("clientids | ", clientids)
			c1 := clientids[0]
			c3 := clientids[1]
			c2 := clientids[2]

			dancerNo := strings.Split(pos.DancerNo, " ")
			d1, _ := strconv.Atoi(dancerNo[0])
			d2, _ := strconv.Atoi(dancerNo[1])
			d3, _ := strconv.Atoi(dancerNo[2])

			posPack1 := posStruct{dancerNo: d1, change: s1, cid: c1}
			posPack2 := posStruct{dancerNo: d2, change: s2, cid: c2}
			posPack3 := posStruct{dancerNo: d3, change: s3, cid: c3}
			packets := []posStruct{posPack1, posPack2, posPack3}
			sort.Slice(packets, func(i, j int) bool { return packets[i].dancerNo < packets[j].dancerNo })

			posPack1 = packets[0]
			posPack2 = packets[1]
			posPack3 = packets[2]

			if !allDancersIdentified {
				dancerNoToPlace[1] = 1
				dancerNoToPlace[2] = 2
				dancerNoToPlace[3] = 3
				allDancersIdentified = true
			}

			// statuses
			// -2 = 2x left
			// -1 = left
			// 0  = stay
			// 1  = right
			// 2  = 2x right

			log.Info("currPos | ", calcPos)

			tempPos1 := dancerNoToPlace[1] + posPack1.change
			tempPos2 := dancerNoToPlace[2] + posPack2.change
			tempPos3 := dancerNoToPlace[3] + posPack3.change

			count := make(map[int]int)

			validPos := []bool{false, false, false}
			var invalidDancers []int
			invalidCount := 0

			if tempPos1 > 0 && tempPos1 < 4 {
				validPos[0] = true
				dancerNoToPlace[1] = tempPos1
			}
			if tempPos2 > 0 && tempPos2 < 4 {
				validPos[1] = true
				dancerNoToPlace[2] = tempPos2
			}
			if tempPos3 > 0 && tempPos3 < 4 {
				validPos[2] = true
				dancerNoToPlace[3] = tempPos3
			}

			for dNo, place := range dancerNoToPlace {
				if count[place] == 0 {
					count[place] = count[place] + 1
					if validPos[dNo-1] {
						places[place] = dNo
					} else {
						invalidDancers = append(invalidDancers, dNo)
						invalidCount++
					}
				} else {
					invalidDancers = append(invalidDancers, dNo)
					invalidCount++
				}
			}

			log.Info("invalidDancers | ", invalidDancers)

			if invalidCount > 0 {
				rand.Shuffle(len(invalidDancers), func(i, j int) { invalidDancers[i], invalidDancers[j] = invalidDancers[j], invalidDancers[i] })

				ptr := 0
				for x, dancer := range places {
					if dancer == 0 {
						places[x] = invalidDancers[ptr]
						dancerNoToPlace[invalidDancers[ptr]] = x
						ptr++
					}

				}
			}

			calcPos = fmt.Sprint(places[1], places[2], places[3])
			log.Info("CalcPos | ", calcPos)

		case delay := <-delayChan:
			recvDelay = delay
		}
	}
}

func startHTTPServer() {
	requestHandler := func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Printf("Error reading body: %v", err)
				http.Error(w, "bad", http.StatusBadRequest)
				return
			}
			io.WriteString(w, "ok")
			log.Debug("Body | ", string(body))

			data := body

			dataChannel <- []byte(AESEncrypt(Pad([]byte(data), aes.BlockSize)))

			reqBody, err := json.Marshal(map[string]string{
				"data": string(data),
			})

			if err != nil {
				log.Error(err)
			} else {
				resp, err := http.Post(dashConnString,
					"application/json", bytes.NewBuffer(reqBody))
				if err != nil {
					log.Error(err)
					log.Error("Could not post to dashconnection on ", dashConnString, " but continuing silently")
				} else {
					defer resp.Body.Close()
					respBody, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						log.Error(err)
					} else {
						log.Info(string(respBody))
					}
				}
			}

		}

	}

	moveHandler := func(w http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		io.WriteString(w, "ok")

		var moveBod = moveBody{}
		err = json.Unmarshal(body, &moveBod)
		if err != nil {
			log.Error("Error unmarshaling move json")
		}
		moveChan <- moveBod
		log.Info("Recv move | ", moveBod.Move)
	}

	delayHandler := func(w http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		io.WriteString(w, "ok")
		var delayBod = delayBody{}
		err = json.Unmarshal(body, &delayBod)
		if err != nil {
			log.Error("Error unmarshaling delay json")
		}
		log.Info("Recv delay | ", delayBod.Delay)
		delayChan <- delayBod.Delay // Blocking send to delayChan (Should be fine as estimated 1 post / sec)

	}

	posHandler := func(w http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		io.WriteString(w, "ok")

		var posBod = posBody{}
		err = json.Unmarshal(body, &posBod)
		if err != nil {
			log.Error("Error unmarshaling position json")
		}
		log.Info("Recv cids | ", posBod.Cids)
		log.Info("Recv position changes | ", posBod.Changes)

		posChan <- posBod
	}

	if mode != "single" {
		http.HandleFunc("/delay", delayHandler)   // For receiving syncdelay in seconds ex 3.141 between fastest & slowest dancer from DataSub using HTTP Post
		http.HandleFunc("/positions", posHandler) // For receiving positions data using HTTP Post
		http.HandleFunc("/move", moveHandler)     // For receiving move ex: rocket , currently needs to receive three times for multi dancer
	}
	go updateRoutine()
	http.HandleFunc("/", requestHandler) // For receiving full string ex: #1 2 3|rocket|0.123 , only used for single dancer , To be migrated to /move
	log.Fatal(http.ListenAndServe("127.0.0.1:10202", nil))
}

func init() {
	flag.StringVar(&connectionString, "conn", "127.0.0.1:12345", "please enter <ip>:<port> of eval server, for example: -conn=127.0.0.1:12345")
	flag.StringVar(&dashConnString, "dashconn", "http://127.0.0.1:3000/api/prediction/", "please enter http://<ip>:<port>/path of dashboard server, for example: -dashconn=http://127.0.0.1:3000/api/prediction/")
	flag.StringVar(&mode, "mode", "single", "Enter mode: single/multi/standalone, defaults to single. Standalone mode runs multi, use it for testing without sending to EvalServer")
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}
func main() {
	flag.Parse()

	log.Info("Starting in ", mode, " mode")
	if mode != "standalone" {
		clientStart()
	}

	rand.Seed(time.Now().UnixNano())
	startHTTPServer()

	//sample := "#1 2 3|rocket|0.12"

}
