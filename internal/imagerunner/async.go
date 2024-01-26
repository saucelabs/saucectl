package imagerunner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/gorilla/websocket"
)

type AsyncEvent struct {
	Type         string
	LineSequence string
	Data         map[string]string
}

type AsyncEventTransportI interface {
	ReadMessage() (string, error)
	Close() error
}

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

type SseAsyncEventTransport struct {
	httpResponse *http.Response
	scanner      *bufio.Scanner
}

func (aet *SseAsyncEventTransport) ReadMessage() (string, error) {
	if aet.scanner.Scan() {
		msg := aet.scanner.Bytes()
		return string(msg), nil
	}
	err := aet.scanner.Err()
	if err == nil {
		err = fmt.Errorf("no more messages")
	}
	return "", err
}

func (aet *SseAsyncEventTransport) Close() error {
	return aet.httpResponse.Body.Close()
}

type AsyncEventManagerI interface {
	ParseEvent(event string) (*AsyncEvent, error)
	TrackLog()
	IsLogIdle() bool
}

type AsyncEventManager struct {
	logTimestamps time.Time
}

func NewAsyncEventManager() (*AsyncEventManager, error) {
	asyncEventManager := AsyncEventManager{
		logTimestamps: time.Now(),
	}

	return &asyncEventManager, nil
}

func parseLineSequence(cloudEvent *cloudevents.Event) (string, error) {
	// The extension is not necessarily present, so ignore errors.
	_lineseq, _ := cloudEvent.Context.GetExtension("linesequence")
	lineseq, ok := _lineseq.(string)
	if !ok {
		return "", fmt.Errorf("linesequence is not a string")
	}
	return lineseq, nil
}

func (a *AsyncEventManager) ParseEvent(event string) (*AsyncEvent, error) {
	readEvent := cloudevents.NewEvent()
	err := json.Unmarshal([]byte(event), &readEvent)
	if err != nil {
		return nil, err
	}

	data := map[string]string{}
	err = readEvent.DataAs(&data)
	if err != nil {
		return nil, err
	}

	asyncEvent := AsyncEvent{
		Type: readEvent.Type(),
		Data: data,
	}

	if asyncEvent.Type == "com.saucelabs.so.v1.log" {
		asyncEvent.LineSequence, err = parseLineSequence(&readEvent)
		if err != nil {
			return nil, err
		}
	}

	return &asyncEvent, nil
}

func (a *AsyncEventManager) TrackLog() {
	a.logTimestamps = time.Now()
}

func (a *AsyncEventManager) IsLogIdle() bool {
	return time.Since(a.logTimestamps) > 30*time.Second
}
