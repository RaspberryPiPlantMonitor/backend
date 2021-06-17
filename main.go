package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	gorillaWs "github.com/gorilla/websocket"
	"github.com/iris-contrib/middleware/cors"
	"github.com/joho/godotenv"
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

func validPassword(ctx iris.Context) error {
	password := os.Getenv("APP_PASSWORD")
	if ctx.URLParamDefault("password", "Not a password") != password {
		return errors.New("Invalid token!")
	}
	return nil
}

func authMiddleware(ctx iris.Context) {
	if err := validPassword(ctx); err != nil {
		ctx.StopWithError(iris.StatusBadRequest, err)
	}
	ctx.Next()
	return
}

func main() {

	// Loading .env vars file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	humiditySensorLimitEnv := "APP_HUMIDITY_SENSOR_LIMIT"
	lightStatusEnv := "APP_LIGHT_STATUS"
	pumpStatusEnv := "APP_PUMP_STATUS"

	var lightRelayStatus byte
	if os.Getenv(lightStatusEnv) == "on" {
		lightRelayStatus = '1'
	} else {
		lightRelayStatus = '0'
	}

	var pumpRelayStatus byte
	if os.Getenv(pumpStatusEnv) == "on" {
		pumpRelayStatus = '1'
	} else {
		pumpRelayStatus = '0'
	}

	// For dev use only
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
		ctx := websocket.GetContext(c)
		if err := validPassword(ctx); err != nil {
			c.Close()
			return err
		}
		log.Printf("[%s] Connected to server!", c.ID())
		return nil
	}

	websocketServer.OnDisconnect = func(c *websocket.Conn) {
		log.Printf("[%s] Disconnected from server", c.ID())
	}

	websocketServer.OnUpgradeError = func(err error) {
		log.Printf("Upgrade Error: %v", err)
	}

	sensorChannel := make(chan []byte)
	defer close(sensorChannel)

	// Sensors stream
	go func(channel chan []byte) {
		mode := &serial.Mode{
			BaudRate: 9600, // Same as Arduino code,
		}
		port, err := serial.Open("/dev/ttyACM0", mode)
		if err != nil {
			log.Fatal(err)
		}

		// First position representes the Light Relay
		// Second position represents the Pump Relay
		outputBuffer := <-channel

		inputBuffer := make([]byte, 4)
		var inputJSON []byte

		for {
			_, outputBufferErr := port.Write(outputBuffer)
			if outputBufferErr != nil {
				log.Fatal(outputBufferErr.Error())
				break
			}

			n, err := port.Read(inputBuffer)
			if err != nil {
				log.Fatal(err)
				break
			}
			if n == 0 {
				fmt.Println("\nEOF")
				break
			}

			for _, b := range inputBuffer[:n] {
				if b == '{' {
					inputJSON = nil
				} else if b == '}' {
					inputJSON = append(inputJSON, b)
					message := websocket.Message{
						Body:     inputJSON,
						IsNative: true,
					}
					websocketServer.Broadcast(nil, message)
					inputJSON = nil
				}
				inputJSON = append(inputJSON, b)
			}
		}
	}(sensorChannel)

	var outputBuffer []byte
	outputBuffer = append(outputBuffer, lightRelayStatus)
	outputBuffer = append(outputBuffer, pumpRelayStatus)

	sensorChannel <- outputBuffer

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
	app.Get("/realtime", websocket.Handler(websocketServer))

	// For dev use only
	crs := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	})
	app.Use(crs)
	app.AllowMethods(iris.MethodOptions)

	type HumidityBody struct {
		Value int
	}

	app.Post("/humiditySensorLimit", authMiddleware, func(ctx iris.Context) {
		humidityBody := HumidityBody{Value: 400}
		err := ctx.ReadJSON(&humidityBody)
		if err != nil {
			ctx.StopWithError(iris.StatusBadRequest, err)
			return
		}
		os.Setenv(humiditySensorLimitEnv, strconv.Itoa(humidityBody.Value))
		ctx.JSON(iris.Map{
			"value": os.Getenv(humiditySensorLimitEnv),
		})
	})

	app.Get("/humiditySensorLimit", authMiddleware, func(ctx iris.Context) {
		if value, ok := os.LookupEnv(humiditySensorLimitEnv); ok {
			ctx.JSON(iris.Map{
				"value": value,
			})
			return
		}
		ctx.JSON(iris.Map{
			"value": "400",
		})
	})

	type LightRelayBody struct {
		Value string
	}

	app.Post("/lightRelay", authMiddleware, func(ctx iris.Context) {
		lightRelayBody := LightRelayBody{Value: ""}
		err := ctx.ReadJSON(&lightRelayBody)
		if err != nil {
			ctx.StopWithError(iris.StatusBadRequest, err)
			return
		}
		if lightRelayBody.Value == "on" {
			lightRelayStatus = '1'
		} else {
			lightRelayStatus = '0'
		}
		outputBuffer[0] = lightRelayStatus
		sensorChannel <- outputBuffer

		ctx.JSON(iris.Map{
			"value": lightRelayStatus,
		})
	})

	app.Run(iris.Addr(":8080"))
}
