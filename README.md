# Plant Monitor System

## Demo video

* Want to see the system in action? Watch this video: 

[![Plant Monitor System Video](https://img.youtube.com/vi/XOIkH0mkq2M/0.jpg)](https://www.youtube.com/watch?v=XOIkH0mkq2M)

## Circuit Diagram

![circuit_setup](circuit_diagram_v1.png) 

**List of components:**
* [Raspberry Pi 3 or greater](https://www.amazon.ca/CanaKit-Raspberry-Starter-Premium-Black/dp/B07BCC8PK7/ref=sr_1_5?dchild=1&keywords=raspberry+pi&qid=1624346134&sr=8-5)
* [Arduino Uno](https://www.amazon.ca/gp/product/B087DYDNGZ/ref=ppx_yo_dt_b_asin_title_o04_s01?ie=UTF8&psc=1)
* [Protoboard](https://www.amazon.ca/Breadboard-Solderless-Prototype-Distribution-Connecting/dp/B01EV6LJ7G/ref=sr_1_14?dchild=1&keywords=Protoboard&qid=1624346215&sr=8-14)
* [Relay 1](https://www.amazon.ca/Iot-Relay-Enclosed-High-power-Raspberry/dp/B00WV7GMA2/ref=sr_1_5?dchild=1&keywords=iot+relay&qid=1624346262&sr=8-5) 
* [Relay 2](https://www.amazon.ca/gp/product/B087DYDNGZ/ref=ppx_yo_dt_b_asin_title_o04_s01?ie=UTF8&psc=1)
* [Moisture Sensor](https://www.amazon.ca/gp/product/B087DYDNGZ/ref=ppx_yo_dt_b_asin_title_o04_s01?ie=UTF8&psc=1)
* [Water Pump](https://www.amazon.ca/gp/product/B087DYDNGZ/ref=ppx_yo_dt_b_asin_title_o04_s01?ie=UTF8&psc=1)
* [USB Camera](https://www.amazon.ca/Logitech-C920-Pro-Webcam-Black/dp/B00829D0GM/ref=sr_1_12?dchild=1&keywords=logitech+camera&qid=1624346663&sr=8-12)
* [Jumper Wires](https://www.amazon.ca/gp/product/B087DYDNGZ/ref=ppx_yo_dt_b_asin_title_o04_s01?ie=UTF8&psc=1)

## Arduino Setup

**[WARNING]** The following instructions were done in a Arduino Uno

* Install the [Arduino IDE](https://www.arduino.cc/en/software) in your computer

* Connect your Arduino Uno via USB to your computer

* Copy the code located [here](https://github.com/RaspberryPiPlantMonitor/arduino/blob/master/code/code.ino) and upload it to your Arduino Uno
    * For a step by step tutorial on how to upload code to your Arduino watch: https://www.youtube.com/watch?v=y5znFDmY5V4&ab_channel=talofer99

## Raspberry Pi Setup

**[WARNING]** The following instructions were done in a RaspberryPi 3

* Open the terminal inside your Raspberry Pi

* Update your Raspberry Pi
    * `sudo apt update`

* Install NodeJS 10 or greater:
    * Follow the steps [here](https://linuxize.com/post/how-to-install-node-js-on-raspberry-pi/)

* Install the Go lang compiler:
    * Follow the steps [here](https://pimylifeup.com/raspberry-pi-golang/)

* Install GoCV:
    * Follow the steps [here](https://github.com/hybridgroup/gocv)

* Create a `workspace` folder: 
    * `mkdir ~/Desktop/workspace`

* Navigate to the `workspace` folder: 
    * `cd ~/Desktop/workspace`

* Clone [this repository](https://github.com/RaspberryPiPlantMonitor/backend): 
    * `git clone https://github.com/RaspberryPiPlantMonitor/backend`

* Navigate inside the `backend` folder: 
    * `cd ~/Desktop/workspace/backend`

* Install project dependencies: 
    * `go mod tidy`

* Navigate back to the `workspace` folder: 
    * `cd ~/Desktop/workspace`

* Clone the [frontend repository](https://github.com/RaspberryPiPlantMonitor/frontend): 
    * `git clone https://github.com/RaspberryPiPlantMonitor/frontend`

* Navigate inside the `frontend` folder:
    * `cd ~/Desktop/workspace/frontend`

* Build the `frontend` code and move the `build` directory to the `backend` folder
    * `npm run build`
    * `mv ~/Desktop/workspace/frontend/build ~/Desktop/workspace/backend/build`

* Navigate back to `backend`:
    * `cd ~/Desktop/workspace/backend`

* Change the `~/Desktop/workspace/backend/.env` file with the setting you desire

### Running the Raspberry Pi server

* Install [ngrok](https://ngrok.com/)
    * `sudo apt install unzip`
    * `cd ~/Desktop`
    * `wget https://bin.equinox.io/c/4VmDzA7iaHb/ngrok-stable-linux-arm.zip`
    * `unzip ngrok-stable-linux-arm.zip`

* Run `ngrok`:
    * `nohup ./ngrok http 8080 > ngrok.log &`

* Navigate to `backend` folder:
    * `cd ~/Desktop/workspace/backend`

* Run the server:
    * `nohup go run main.go > ngrok.log &`

* Check your ngrok endpoint by doing:
    * `curl http://localhost:4040/api/tunnels`
    * Copy the `public_url` url. It starts with `https`

## Extra commands

* Check server logs:
    * `tail -f ~/Desktop/workspace/backend/ngrok.log`

* Check your `nohups` jobs by running:
    * `jobs -l`






