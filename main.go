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
	"github.com/joho/godotenv"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/websocket"
	"github.com/kataras/neffos"
	"github.com/kataras/neffos/gorilla"
	"go.bug.st/serial"
	"gocv.io/x/gocv"
)

// ngrok http -auth="username:password" 8080
// nohup go run main.go > ngrok.log &
// nohup ./ngrok http 8080 > ngrok.log &
// curl http://localhost:4040/api/tunnels
// jobs

func validPassword(password string) error {
	correctPassword := os.Getenv("APP_PASSWORD")
	if password != correctPassword {
		return errors.New("Invalid token!")
	}
	return nil
}

func authMiddleware(ctx iris.Context) {
	type authHeader struct {
		Authorization string `header:"Authorization,required"`
	}
	var authHeaders authHeader
	if err := ctx.ReadHeaders(&authHeaders); err != nil {
		ctx.StopWithError(iris.StatusInternalServerError, err)
		return
	}

	if err := validPassword(authHeaders.Authorization); err != nil {
		ctx.StopWithError(iris.StatusBadRequest, err)
		return
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

	humiditySensorLimitEnvVar := "APP_HUMIDITY_SENSOR_LIMIT"
	lightStatusEnvVar := "APP_LIGHT_STATUS"
	pumpStatusEnvVar := "APP_PUMP_STATUS"
	enableCORSEnvVar := "APP_ENABLE_CORS"
	portEnvVar := "APP_PORT"

	var lightRelayStatus byte
	if os.Getenv(lightStatusEnvVar) == "on" {
		lightRelayStatus = '1'
	} else {
		lightRelayStatus = '0'
	}

	var pumpRelayStatus byte
	if os.Getenv(pumpStatusEnvVar) == "on" {
		pumpRelayStatus = '1'
	} else {
		pumpRelayStatus = '0'
	}

	var upgrader neffos.Upgrader = nil

	if os.Getenv(enableCORSEnvVar) == "true" {
		// For dev use only
		upgrader = gorilla.Upgrader(gorillaWs.Upgrader{
			CheckOrigin: func(*http.Request) bool {
				return true
			}})
	}

	websocketServer := websocket.New(upgrader, websocket.Events{
		websocket.OnNativeMessage: func(nsConn *websocket.NSConn, msg websocket.Message) error {
			log.Printf("Server got: %s from [%s]", msg.Body, nsConn.Conn.ID())
			return nil
		},
	})

	websocketServer.OnConnect = func(c *websocket.Conn) error {
		ctx := websocket.GetContext(c)

		if err := validPassword(ctx.URLParamDefault("password", "")); err != nil {
			c.Close()
			ctx.StopWithError(iris.StatusInternalServerError, err)
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

	// Initialize sensor data
	var sensorOptionsBuffer []byte
	sensorOptionsBuffer = append(sensorOptionsBuffer, lightRelayStatus)
	sensorOptionsBuffer = append(sensorOptionsBuffer, pumpRelayStatus)

	// Sensors stream
	go func(channel chan []byte) {
		mode := &serial.Mode{
			BaudRate: 9600, // Same as Arduino code,
		}
		port, err := serial.Open("/dev/ttyACM0", mode)
		defer port.Close()

		if err != nil {
			log.Fatal(err)
		}

		// First position representes the Light Relay
		// Second position represents the Pump Relay
		outputBuffer := <-channel

		inputBuffer := make([]byte, 4)
		var inputJSON []byte

		for {

			select {
			case outputBuffer = <-channel:
				fmt.Printf("Changed switch: %v\n", outputBuffer)
			default:
				//fmt.Println("no message received")
			}

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

	sensorChannel <- sensorOptionsBuffer

	// Video stream
	go func() {
		camera, err := gocv.VideoCaptureDevice(0)
		if err != nil {
			panic(err)
		}
		defer camera.Close()

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
			time.Sleep(time.Second / 2)
		}
	}()

	app := iris.New()

	app.HandleDir("/", iris.Dir("./build"))

	app.Get("/realtime", websocket.Handler(websocketServer))

	if os.Getenv(enableCORSEnvVar) == "true" {
		// Our custom CORS middleware.
		crs := func(ctx iris.Context) {
			ctx.Header("Access-Control-Allow-Origin", "http://localhost:3001")
			ctx.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,Sec-WebSocket-Protocol")
			ctx.Header("Access-Control-Allow-Methods", "GET,PUT,POST,DELETE,OPTIONS")
			ctx.Header("Access-Control-Allow-Credentials", "true")

			if ctx.Method() == iris.MethodOptions {
				ctx.Header("Access-Control-Methods",
					"POST, PUT, PATCH, DELETE")

				ctx.Header("Access-Control-Allow-Headers",
					"Access-Control-Allow-Origin,Content-Type,Authorization,Sec-WebSocket-Protocol")

				ctx.Header("Access-Control-Max-Age",
					"86400")

				ctx.StatusCode(iris.StatusNoContent)
				return
			}
			ctx.Next()
		}

		app.UseRouter(crs)
	}

	app.Post("/humiditySensorLimit", authMiddleware, func(ctx iris.Context) {
		type HumidityBody struct {
			Value int
		}

		humidityBody := HumidityBody{Value: 400}

		err := ctx.ReadJSON(&humidityBody)
		if err != nil {
			ctx.StopWithError(iris.StatusBadRequest, err)
			return
		}
		os.Setenv(humiditySensorLimitEnvVar, strconv.Itoa(humidityBody.Value))
		ctx.JSON(iris.Map{
			"value": os.Getenv(humiditySensorLimitEnvVar),
		})
	})

	app.Get("/humiditySensorLimit", authMiddleware, func(ctx iris.Context) {
		if value, ok := os.LookupEnv(humiditySensorLimitEnvVar); ok {
			ctx.JSON(iris.Map{
				"value": value,
			})
			return
		}
		ctx.JSON(iris.Map{
			"value": "400",
		})
	})

	app.Post("/lightRelay", authMiddleware, func(ctx iris.Context) {
		type LightRelayBody struct {
			Value string
		}

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
		sensorOptionsBuffer[0] = lightRelayStatus
		sensorChannel <- sensorOptionsBuffer

		// Wait a bit for update to propagate on the Arduino
		time.Sleep(time.Second * 3)

		ctx.JSON(iris.Map{
			"value": lightRelayStatus,
		})
	})

	app.Post("/pumpRelay", authMiddleware, func(ctx iris.Context) {
		type PumpRelayBody struct {
			Value string
		}

		pumpRelayBody := PumpRelayBody{Value: ""}

		err := ctx.ReadJSON(&pumpRelayBody)
		if err != nil {
			ctx.StopWithError(iris.StatusBadRequest, err)
			return
		}
		if pumpRelayBody.Value == "on" {
			pumpRelayStatus = '1'
		} else {
			pumpRelayStatus = '0'
		}
		sensorOptionsBuffer[1] = pumpRelayStatus
		sensorChannel <- sensorOptionsBuffer

		// Wait a bit for update to propagate on the Arduino
		time.Sleep(time.Second * 3)

		ctx.JSON(iris.Map{
			"value": pumpRelayStatus,
		})
	})

	app.Run(iris.Addr(":" + os.Getenv(portEnvVar)))
}
