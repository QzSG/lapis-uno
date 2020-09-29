package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"
)

const key = "testtesttesttest"

var dataChannel = make(chan []byte)
var connectionString string

// Client : TCP Client
type Client struct {
	sock net.Conn
}

func (client *Client) recv() {
	for {
		msg := make([]byte, 4096)
		len, err := client.sock.Read(msg)
		if err != nil {
			client.sock.Close()
			break
		}
		if len > 0 {
			fmt.Println("RECEIVED: " + string(msg))
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

/*func (client *Client) send(data []byte) {
	(*client).sock.Write(data)
}*/

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
	rand.Seed(time.Now().UnixNano())
	iv := make([]byte, 16)

	if _, err := rand.Read(iv); err != nil {
		fmt.Println("Error occured generating IV")
	}

	return iv
}

// AESEncrypt : Encrypt data with AES-128-CBC
func AESEncrypt(data []byte) string {
	iv := generateIV()
	ciph, err := aes.NewCipher([]byte(key))
	if err != nil {
		fmt.Println("Error with the key: ", err)
	}
	encrypter := cipher.NewCBCEncrypter(ciph, iv)
	ciphertext := make([]byte, len(iv)+aes.BlockSize)
	encrypter.CryptBlocks(ciphertext, data)

	ivData := append(iv, ciphertext...)
	return encode(ivData)
}

func clientStart() { //Client {
	fmt.Println("Lapis Comms Client Starting...")
	conn, err := net.Dial("tcp", connectionString)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("Lapis Comms Client connected.")
	}
	client := &Client{sock: conn}
	go client.recv()
	go client.send()
	//return *client
}

func consoleListenerStart() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		dataChannel <- []byte(AESEncrypt(Pad(scanner.Bytes(), aes.BlockSize)))
	}
}

func init() {
	flag.StringVar(&connectionString, "conn", "127.0.0.1:12345", "please enter <ip>:<port>, for example: -conn 127.0.0.1:12345")
}
func main() {
	flag.Parse()
	println(connectionString)
	clientStart()
	consoleListenerStart()

	//sample := "#1 2 3|rocket|0.12"
	//client.data <- []byte(AESEncrypt(Pad([]byte(sample), aes.BlockSize)))
	//client.data <- []byte(AESEncrypt(Pad([]byte(sample), aes.BlockSize)))
	//client.data <- []byte(AESEncrypt(Pad([]byte(sample), aes.BlockSize)))
	//client.data <- []byte(AESEncrypt(Pad([]byte(sample), aes.BlockSize)))
	//paddedSample := Pad([]byte(sample), aes.BlockSize)
	//fmt.Println("{" + string(paddedSample) + "}")
	//fmt.Println("{" + strings.TrimSpace(string(paddedSample)) + "}")

	//client := clientStart()
	//sample := "#1 2 3|rocket|0."
	/*time.Sleep(3000 * time.Millisecond)
	for i := 1; i < 50; i++ {
		mod := sample + strconv.Itoa(i)
		println(mod)
		client.send([]byte(AESEncrypt(Pad([]byte(mod), aes.BlockSize))))
	}*/
}
