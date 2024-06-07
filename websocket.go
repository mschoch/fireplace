package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func newWebsocketHandler(ts Service, logger log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Log("msg", "error upgrading websocket", "err", err)
			return
		}

		wsConn := &WSConnection{send: make(chan interface{}, 256), ws: conn}
		ts.RegisterMeta(wsConn.send)
		defer ts.UnregisterMeta(wsConn.send)

		go wsConn.Writer()
		wsConn.Reader()
	}
}

type WSConnection struct {
	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
	send chan interface{}
}

func (c *WSConnection) Reader() {
	for {
		if _, msg, err := c.ws.ReadMessage(); err == nil {
			parsedMessage := map[string]interface{}{}
			err = json.Unmarshal(msg, &parsedMessage)
			if err != nil {
				fmt.Printf("error decoding message json %v\n", err)
				continue
			}

		} else {
			break
		}
	}
	c.ws.Close()
}

func (c *WSConnection) Writer() {
	for message := range c.send {
		bytes, err := json.Marshal(message)
		if err != nil {
			fmt.Printf("Failed to marshall notification to JSON, ignoring: %v\n", err)
			continue
		}
		err = c.ws.WriteMessage(websocket.TextMessage, bytes)
		if err != nil {
			break
		}
	}
	c.ws.Close()
}
