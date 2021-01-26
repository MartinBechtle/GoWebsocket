package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/MartinBechtle/go-websocket"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	socket := gowebsocket.New("wss://fstream.binance.com/stream?streams=btcusdt@aggTrade/ethusdt@aggTrade")
	socket.Timeout = 3 * time.Second
	socket.ReconnectionOptions = gowebsocket.ReconnectionOptions{
		RetryOnConnectionError:      true,
		RetryDelayOnConnectionError: 5 * time.Second,
		ReconnectOnDisconnection:    true,
		ReconnectDelay:              3 * time.Second,
	}

	socket.OnConnectError = func(err error, socket gowebsocket.Socket) {
		log.Println("Error connecting to ByBit websocket, will attempt reconnection in 5s. Error was:", err)
	}
	socket.OnConnected = func(socket gowebsocket.Socket) {
		log.Println("Connected to Binance websocket")
	}
	socket.OnTextMessage = func(message string, socket gowebsocket.Socket) {
		fmt.Println(message)
	}
	socket.OnDisconnected = func(err error, socket gowebsocket.Socket) {
		log.Println("Disconnected from server, will attempt reconnection in 3s")
	}

	socket.Connect()
	defer socket.Close()

	<-interrupt
}
