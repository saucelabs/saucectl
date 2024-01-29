package imagerunner

import (
	"encoding/json"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type AsyncEvent struct {
	Type         string
	LineSequence string
	Data         map[string]string
}

type AsyncEventTransporter interface {
	ReadMessage() (string, error)
	Close() error
}

type AsyncEventManager interface {
	ParseEvent(event string) (*AsyncEvent, error)
	IsLogIdle() bool
}

type AsyncEventMgr struct {
	lastLogTime time.Time
}

func NewAsyncEventMgr() (*AsyncEventMgr, error) {
	asyncEventManager := AsyncEventMgr{
		lastLogTime: time.Now(),
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

func (a *AsyncEventMgr) ParseEvent(event string) (*AsyncEvent, error) {
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
		a.lastLogTime = time.Now()
	}

	return &asyncEvent, nil
}

func (a *AsyncEventMgr) IsLogIdle() bool {
	return time.Since(a.lastLogTime) > 30*time.Second
}
