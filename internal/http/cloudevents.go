package http

import "github.com/gorilla/websocket"

type WebSocketAsyncEventTransport struct {
	ws *websocket.Conn
}

func NewWebSocketAsyncEventTransport(ws *websocket.Conn) *WebSocketAsyncEventTransport {
	return &WebSocketAsyncEventTransport{
		ws: ws,
	}
}

func (aet *WebSocketAsyncEventTransport) ReadMessage() (string, error) {
	_, msg, err := aet.ws.ReadMessage()
	return string(msg), err
}

func (aet *WebSocketAsyncEventTransport) Close() error {
	return aet.ws.Close()
}
