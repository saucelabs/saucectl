package imagerunner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"

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

type WebsocketAsyncEventTransport struct {
	ws *websocket.Conn
}

func NewWebsocketAsyncEventTransport(ws *websocket.Conn) *WebsocketAsyncEventTransport {
	return &WebsocketAsyncEventTransport{
		ws: ws,
	}
}

func (aet *WebsocketAsyncEventTransport) ReadMessage() (string, error) {
	_, msg, err := aet.ws.ReadMessage()
	return string(msg), err
}

func (aet *WebsocketAsyncEventTransport) Close() error {
	return aet.ws.Close()
}

type SseAsyncEventTransport struct {
	httpResponse *http.Response
	scanner      *bufio.Scanner
}

func NewSseAsyncEventTransport(httpResponse *http.Response) *SseAsyncEventTransport {
	scanner := bufio.NewScanner(httpResponse.Body)
	scanner.Split(bufio.ScanLines)
	return &SseAsyncEventTransport{
		httpResponse: httpResponse,
		scanner:      scanner,
	}
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
}

type AsyncEventManager struct {
}

func NewAsyncEventManager() (*AsyncEventManager, error) {
	asyncEventManager := AsyncEventManager{}

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
