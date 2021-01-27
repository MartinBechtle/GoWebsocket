package gowebsocket

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Socket struct {
	Conn            *websocket.Conn
	WebsocketDialer *websocket.Dialer
	Url             string
	Connection      ConnectionOptions
	Reconnection    ReconnectionOptions
	RequestHeader   http.Header
	OnConnected     func(socket Socket)
	OnTextMessage   func(message string, socket Socket)
	OnBinaryMessage func(data []byte, socket Socket)
	Log             Logger
	Timeout         time.Duration
	OnConnectError  func(err error, socket Socket)
	OnDisconnected  func(err error, socket Socket)
	OnPingReceived  func(data string, socket Socket)
	OnPongReceived  func(data string, socket Socket)
	connected       bool
	sendMu          *sync.Mutex // Prevent "concurrent write to websocket connection"
	receiveMu       *sync.Mutex
}

type ConnectionOptions struct {
	UseCompression bool
	UseSSL         bool
	Proxy          func(*http.Request) (*url.URL, error)
	SubProtocols   []string
	AutoRetry      bool
	RetryDelay     time.Duration
}

type ReconnectionOptions struct {
	AutoRetry  bool
	RetryDelay time.Duration
}

func New(url string) Socket {
	return Socket{
		Url:           url,
		RequestHeader: http.Header{},
		Connection: ConnectionOptions{
			UseCompression: false,
			UseSSL:         true,
			AutoRetry:      false,
		},
		Reconnection: ReconnectionOptions{
			AutoRetry: false,
		},
		WebsocketDialer: &websocket.Dialer{},
		Log:             NoOpLogger{},
		Timeout:         1 * time.Minute,
		sendMu:          &sync.Mutex{},
		receiveMu:       &sync.Mutex{},
	}
}

func (socket *Socket) setConnectionOptions() {
	socket.WebsocketDialer.EnableCompression = socket.Connection.UseCompression
	socket.WebsocketDialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: socket.Connection.UseSSL}
	socket.WebsocketDialer.Proxy = socket.Connection.Proxy
	socket.WebsocketDialer.Subprotocols = socket.Connection.SubProtocols
}

func (socket *Socket) Connect() {
	var err error
	var resp *http.Response
	socket.setConnectionOptions()

	socket.Conn, resp, err = socket.WebsocketDialer.Dial(socket.Url, socket.RequestHeader)

	if err != nil {
		socket.Log.Errorf("Error while connecting: %v", err)
		if resp != nil {
			socket.Log.Errorf("HTTP Response %d status: %s", resp.StatusCode, resp.Status)
		}
		socket.connected = false
		if socket.OnConnectError != nil {
			socket.OnConnectError(err, *socket)
		}
		socket.maybeRetryConnection()
		return
	}

	socket.Log.Infof("Connected")
	socket.connected = true

	if socket.OnConnected != nil {
		socket.OnConnected(*socket)
	}

	defaultPingHandler := socket.Conn.PingHandler()
	socket.Conn.SetPingHandler(func(appData string) error {
		socket.Log.Tracef("Received PING from server")
		if socket.OnPingReceived != nil {
			socket.OnPingReceived(appData, *socket)
		}
		return defaultPingHandler(appData)
	})

	defaultPongHandler := socket.Conn.PongHandler()
	socket.Conn.SetPongHandler(func(appData string) error {
		socket.Log.Infof("Received PONG from server")
		if socket.OnPongReceived != nil {
			socket.OnPongReceived(appData, *socket)
		}
		return defaultPongHandler(appData)
	})

	defaultCloseHandler := socket.Conn.CloseHandler()
	socket.Conn.SetCloseHandler(func(code int, text string) error {
		result := defaultCloseHandler(code, text)
		socket.Log.Warnf("Disconnected from server: %v", result)
		socket.connected = false
		if socket.OnDisconnected != nil {
			socket.OnDisconnected(errors.New(text), *socket)
		}
		socket.maybeScheduleReconnection()
		return result
	})

	go func() {
		for {
			socket.receiveMu.Lock()
			if socket.Timeout != 0 {
				_ = socket.Conn.SetReadDeadline(time.Now().Add(socket.Timeout))
			}
			messageType, message, err := socket.Conn.ReadMessage()
			socket.receiveMu.Unlock()
			if err != nil {
				socket.Log.Errorf("Couldn't read message:", err)
				socket.connected = false
				if socket.OnDisconnected != nil {
					socket.OnDisconnected(err, *socket)
				}
				socket.maybeScheduleReconnection()
				return
			}
			socket.Log.Debugf("Received message: %s", message)

			switch messageType {
			case websocket.TextMessage:
				if socket.OnTextMessage != nil {
					socket.OnTextMessage(string(message), *socket)
				}
			case websocket.BinaryMessage:
				if socket.OnBinaryMessage != nil {
					socket.OnBinaryMessage(message, *socket)
				}
			}
		}
	}()
}

func (socket *Socket) SendText(message string) {
	err := socket.send(websocket.TextMessage, []byte(message))
	if err != nil {
		socket.Log.Infof("Couldn't write text:", err)
		return
	}
}

func (socket *Socket) SendBinary(data []byte) {
	err := socket.send(websocket.BinaryMessage, data)
	if err != nil {
		socket.Log.Infof("Couldn't write binary:", err)
		return
	}
}

func (socket *Socket) send(messageType int, data []byte) error {
	socket.sendMu.Lock()
	err := socket.Conn.WriteMessage(messageType, data)
	socket.sendMu.Unlock()
	return err
}

func (socket *Socket) Close() {
	err := socket.send(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		socket.Log.Warnf("Closing websocket:", err)
	}
	_ = socket.Conn.Close()
	socket.connected = false
	if socket.OnDisconnected != nil {
		socket.OnDisconnected(err, *socket)
	}
}

func (socket *Socket) IsConnected() bool {
	return socket.connected
}

func (socket *Socket) maybeScheduleReconnection() {
	if socket.Reconnection.AutoRetry {
		delay := socket.Reconnection.RetryDelay
		socket.Log.Debugf("Will attempt reconnection in %s", delay)
		time.AfterFunc(delay, func() {
			socket.Connect()
		})
	}
}

func (socket *Socket) maybeRetryConnection() {
	if socket.Reconnection.AutoRetry {
		delay := socket.Reconnection.RetryDelay
		socket.Log.Debugf("Retrying connection in %s", delay)
		time.AfterFunc(delay, func() {
			socket.Connect()
		})
	}
}
