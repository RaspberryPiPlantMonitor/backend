package main

import (
	"encoding/base64"
	"encoding/json"
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

// Limit pump runtime in "pumpRuntimeLimit" seconds for safety
func setTimer(duration int64, channel chan bool) {
	startTime := time.Now().Unix()
	for {
		currentTime := time.Now().Unix()
		channel <- true
		// Stop timer
		if (currentTime - startTime) > duration {
			channel <- false
			break
		}
	}
}

func main() {

	// Loading .env vars file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	humiditySensorMinEnvVar := "APP_HUMIDITY_SENSOR_MIN"
	pumpRuntimeLimitSecondsEnvVar := "APP_PUMP_RUNTIME_LIMIT_SECONDS"
	lightStatusEnvVar := "APP_LIGHT_STATUS"
	pumpStatusEnvVar := "APP_PUMP_STATUS"
	enableCORSEnvVar := "APP_ENABLE_CORS"
	portEnvVar := "APP_PORT"

	var pumpRuntimeLimitSeconds int = 5
	if len(os.Getenv(pumpRuntimeLimitSecondsEnvVar)) > 0 {
		if value, err := strconv.Atoi(os.Getenv(pumpRuntimeLimitSecondsEnvVar)); err == nil {
			pumpRuntimeLimitSeconds = value
		}
	}

	var lightRelayStatus byte = '0'
	if os.Getenv(lightStatusEnvVar) == "on" {
		lightRelayStatus = '1'
	}

	var pumpRelayStatus byte = '0'
	if os.Getenv(pumpStatusEnvVar) == "on" {
		pumpRelayStatus = '1'
	}

	var websocketUpgrader neffos.Upgrader = websocket.DefaultGorillaUpgrader

	if os.Getenv(enableCORSEnvVar) == "true" {
		// For dev use only
		websocketUpgrader = gorilla.Upgrader(gorillaWs.Upgrader{
			CheckOrigin: func(*http.Request) bool {
				return true
			}})
	}

	websocketServer := websocket.New(websocketUpgrader, websocket.Events{
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

	// Collect arduino sensor data and stream it via Websockets to the client
	go func(sensorChannel chan []byte) {
		mode := &serial.Mode{
			BaudRate: 9600, // Same as Arduino code,
		}
		port, err := serial.Open("/dev/ttyACM0", mode)
		defer port.Close()

		if err != nil {
			log.Fatal(err)
		}

		timerChannel := make(chan bool)
		defer close(timerChannel)
		timerChannelActive := false

		// First position representes the Light Relay
		// Second position represents the Pump Relay
		sensorInputBuffer := <-sensorChannel
		sensorOutputBuffer := make([]byte, 4)

		var sensorOutput []byte
		for {
			select {
			case sensorInputBuffer = <-sensorChannel:
				fmt.Printf("Changed switch: %v\n", sensorInputBuffer)
			case timerChannelActive = <-timerChannel:
				if timerChannelActive {
					sensorInputBuffer[1] = '1'
				} else {
					sensorInputBuffer[1] = '0'
				}
			default:
			}

			_, sensorInputBufferErr := port.Write(sensorInputBuffer)
			if sensorInputBufferErr != nil {
				log.Fatal(sensorInputBufferErr.Error())
				break
			}

			n, sensorOutputBufferErr := port.Read(sensorOutputBuffer)
			if sensorOutputBufferErr != nil {
				log.Fatal(sensorOutputBufferErr)
				break
			}
			if n == 0 {
				fmt.Println("\nEOF")
				break
			}

			for _, b := range sensorOutputBuffer[:n] {
				if b == '{' {
					sensorOutput = nil
				} else if b == '}' {
					sensorOutput = append(sensorOutput, b)

					var sensorOutputJSON map[string]interface{}
					if err := json.Unmarshal(sensorOutput, &sensorOutputJSON); err == nil {

						humidityValue := sensorOutputJSON["humidityValue"].(float64)

						humiditySensorMin, humiditySensorMinErr := strconv.ParseFloat(os.Getenv(humiditySensorMinEnvVar), 64)
						if humiditySensorMinErr != nil {
							humiditySensorMin = 300
						}

						// Water manually
						if !timerChannelActive && sensorInputBuffer[1] == '1' {
							go setTimer(int64(pumpRuntimeLimitSeconds), timerChannel)
							// Water automatically based on parameter baseline
						} else if !timerChannelActive && humidityValue > humiditySensorMin {
							sensorInputBuffer[1] = '1'
							go setTimer(int64(pumpRuntimeLimitSeconds), timerChannel)
						}

						message := websocket.Message{
							Body:     sensorOutput,
							IsNative: true,
						}
						websocketServer.Broadcast(nil, message)
					}
					sensorOutput = nil
				}
				sensorOutput = append(sensorOutput, b)
			}
		}
	}(sensorChannel)

	sensorChannel <- sensorOptionsBuffer

	// Connect to USB/Pi Camera and send base64 images via Websockets to the client
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
			ctx.Header("Access-Control-Allow-Origin", "http://localhost:3000")
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

	app.Post("/humiditySensorMin", authMiddleware, func(ctx iris.Context) {
		type HumidityBody struct {
			Value int
		}

		humidityBody := HumidityBody{Value: 400}

		err := ctx.ReadJSON(&humidityBody)
		if err != nil {
			ctx.StopWithError(iris.StatusBadRequest, err)
			return
		}
		os.Setenv(humiditySensorMinEnvVar, strconv.Itoa(humidityBody.Value))
		ctx.JSON(iris.Map{
			"value": os.Getenv(humiditySensorMinEnvVar),
		})
	})

	app.Get("/humiditySensorMin", authMiddleware, func(ctx iris.Context) {
		if value, ok := os.LookupEnv(humiditySensorMinEnvVar); ok {
			ctx.JSON(iris.Map{
				"value": value,
			})
			return
		}
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
		var lightRelayStatus byte = '0'
		if lightRelayBody.Value == "on" {
			lightRelayStatus = '1'
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
		var pumpRelayStatus byte = '0'
		if pumpRelayBody.Value == "on" {
			pumpRelayStatus = '1'
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
