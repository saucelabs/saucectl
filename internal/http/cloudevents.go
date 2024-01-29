package http

import (
	"encoding/json"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/gorilla/websocket"
	"github.com/saucelabs/saucectl/internal/imagerunner"
)

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

type AsyncEventParser struct {
	lastLogTime time.Time
}

func NewAsyncEventMgr() (*AsyncEventParser, error) {
	asyncEventManager := AsyncEventParser{
		lastLogTime: time.Now(),
	}

	return &asyncEventManager, nil
}

func (a *AsyncEventParser) ParseEvent(event string) (*imagerunner.AsyncEvent, error) {
	e := cloudevents.NewEvent()
	err := json.Unmarshal([]byte(event), &e)
	if err != nil {
		return nil, err
	}

	data := map[string]string{}
	err = e.DataAs(&data)
	if err != nil {
		return nil, err
	}

	asyncEvent := imagerunner.AsyncEvent{
		Type: e.Type(),
		Data: data,
	}

	if asyncEvent.Type == "com.saucelabs.so.v1.log" {
		asyncEvent.LineSequence, err = parseLineSequence(&e)
		if err != nil {
			return nil, err
		}
		a.lastLogTime = time.Now()
	}

	return &asyncEvent, nil
}

func (a *AsyncEventParser) IsLogIdle() bool {
	return time.Since(a.lastLogTime) > 30*time.Second
}

func parseLineSequence(cloudEvent *cloudevents.Event) (string, error) {
	// The extension is not necessarily present, so ignore errors.
	ls, _ := cloudEvent.Context.GetExtension("linesequence")
	lineseq, ok := ls.(string)
	if !ok {
		return "", fmt.Errorf("linesequence is not a string")
	}
	return lineseq, nil
}
