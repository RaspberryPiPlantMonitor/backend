package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"time"

	gorillaWs "github.com/gorilla/websocket"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/websocket"
	"github.com/kataras/neffos/gorilla"
	"go.bug.st/serial"
	"gocv.io/x/gocv"
)

// ngrok http -auth="username:password" 8080
// nohup go run main.go > ngrok.log &
// nohup ./ngrok tcp 8080 > ngrok.log &
// curl http://localhost:4040/api/tunnels

func main() {

	upgrader := gorilla.Upgrader(gorillaWs.Upgrader{
		CheckOrigin: func(*http.Request) bool {
			return true
		}})

	websocketServer := websocket.New(upgrader, websocket.Events{
		websocket.OnNativeMessage: func(nsConn *websocket.NSConn, msg websocket.Message) error {
			log.Printf("Server got: %s from [%s]", msg.Body, nsConn.Conn.ID())
			return nil
		},
	})

	websocketServer.OnConnect = func(c *websocket.Conn) error {
		log.Printf("[%s] Connected to server!", c.ID())
		return nil
	}

	websocketServer.OnDisconnect = func(c *websocket.Conn) {
		log.Printf("[%s] Disconnected from server", c.ID())
	}

	websocketServer.OnUpgradeError = func(err error) {
		log.Printf("Upgrade Error: %v", err)
	}

	// Humidity sensor stream
	go func() {
		mode := &serial.Mode{
			BaudRate: 9600, // Same as Arduino code,
		}
		port, err := serial.Open("/dev/ttyACM0", mode)
		if err != nil {
			log.Fatal(err)
		}

		buffer := make([]byte, 4)
		sensorData := make([]byte, 6)
		counter := 0
		readStream := false

		for {
			buffer = make([]byte, 4)
			n, err := port.Read(buffer)

			if err != nil {
				log.Fatal(err)
				port.Close()
				break
			}
			if n == 0 {
				fmt.Println("No sensor data found!")
				port.Close()
				break
			}

			for _, b := range buffer {
				// Counter = 6 means we have the sensor value XXX.XX (6 characters)
				// Example: 394.00
				if counter == 6 {
					message := websocket.Message{
						Body:     sensorData,
						IsNative: true,
					}
					websocketServer.Broadcast(nil, message)
				}
				// If ":" aka byte 58 was found, start reading stream
				// Stream = ML:111.11\n\r
				if b == byte(58) && !readStream {
					readStream = true
					continue
				}
				if readStream == false {
					continue
				}
				// Ignore bytes that are not numbers (0 to 9) or a dot "."
				// Check ASCII table
				if !(b >= byte(48) && b <= byte(57)) && b != byte(46) {
					sensorData = make([]byte, 6)
					counter = 0
					readStream = false
					continue
				}
				if readStream {
					sensorData[counter] = b
					counter++
				}
			}
		}
	}()

	// Video stream
	go func() {
		camera, err := gocv.VideoCaptureDevice(0)
		if err != nil {
			panic(err)
		}

		image := gocv.NewMat()

		for {
			camera.Read(&image)
			imageData, err := gocv.IMEncode(".jpg", image)
			if err != nil {
				fmt.Println(err)
			} else {
				imageEncodedLen := base64.StdEncoding.EncodedLen(len(imageData))
				imageByteArr := make([]byte, imageEncodedLen)
				base64.StdEncoding.Encode(imageByteArr, imageData)
				urldata := "data:image/jpeg;base64," + string(imageByteArr)
				message := websocket.Message{
					Body:     []byte(urldata),
					IsNative: true,
				}
				websocketServer.Broadcast(nil, message)
			}
			time.Sleep(time.Millisecond * time.Duration(50))
		}
	}()

	app := iris.New()
	app.Get("/", websocket.Handler(websocketServer))

	app.Run(iris.Addr(":8080"))
}
