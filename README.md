# Plant Monitor Backend

**[WARNING]** The following instructions were done in a RaspberryPi 3

## Setup

* Install the [Go compiler](https://pimylifeup.com/raspberry-pi-golang/)

* Install go module package dependencies: `go mod tidy`

* Install OpenCV, it is a dependency of [GoCV](https://github.com/hybridgroup/gocv):
    * ```bash
        cd $GOPATH/src/gocv.io/x/gocv
        make install_raspi
        ```
    
    * If it works correctly, at the end of the entire process, the following message should be displayed:

        * ```bash
            gocv version: 0.27.0
            opencv lib version: 4.5.2
            ```

* Change your `.env` file with the setting you desire

* Build the `frontend` code and move the `build` folder to the root of this folder

### Run it inside the network

* Run `go run main.go`

### Run it on the internet

* Run `nohup go run main.go > ngrok.log &`

* Download [ngrok](https://ngrok.com/) and run `nohup ./ngrok http 8080 > ngrok.log &`

* Check your ngrok endpoint by doing `curl http://localhost:4040/api/tunnels`

* Check your `nohups` jobs by running `jobs`






