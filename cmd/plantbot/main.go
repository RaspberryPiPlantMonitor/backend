package main

import (
	"encoding/base64"
	"fmt"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/websocket"
	"github.com/kataras/neffos/gorilla"
	gorillaWs "github.com/gorilla/websocket"
	"net/http"
	"gocv.io/x/gocv"
	"log"
	"time"
)

// ngrok http -auth="username:password" 8080

func main() {
	upgrader := gorilla.Upgrader(gorillaWs.Upgrader{CheckOrigin: func(*http.Request) bool{return true}})

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
	app.HandleDir("/", "./index.html")
	app.Get("/video", websocket.Handler(websocketServer))

	app.Run(iris.Addr(":8080"))
}