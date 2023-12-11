package imagerunner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

var SCHEMA = `
{
  "properties": {
    "kind": {
      "enum": [
        "notice",
        "log",
        "ping"
      ]
    },
    "runnerID": {
      "type": "string"
    }
  },
  "allOf": [
    {
      "if": {
        "properties": {
          "kind": {
            "const": "log"
          }
        }
      },
      "then": {
        "properties": {
          "lines": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "id": {
                  "type": "string"
                },
                "containerName": {
                  "type": "string"
                },
                "message": {
                  "type": "string"
                }
              }
            }
          }
        },
        "additionalProperties": true
      }
    },
    {
      "if": {
        "properties": {
          "kind": {
            "const": "notice"
          }
        }
      },
      "then": {
        "properties": {
          "severity": {
            "enum": [
              "info",
              "warning",
              "error"
            ]
          },
          "message": {
            "type": "string"
          }
        },
        "additionalProperties": true
      }
    },
    {
      "if": {
        "properties": {
          "kind": {
            "const": "ping"
          }
        }
      },
      "then": {
        "properties": {
          "message": {
            "type": "string"
          }
        },
        "additionalProperties": true
      }
    }

  ],
  "additionalProperties": true
}
`

const (
	NOTICE = "notice"
	LOG    = "log"
	PING   = "ping"
)

type AsyncEventI interface {
	GetKind() string
	GetRunnerID() string
}

type AsyncEvent struct {
	Kind     string `json:"kind"`
	RunnerID string `json:"runnerID"`
}

func (a *AsyncEvent) GetKind() string {
	return a.Kind
}

func (a *AsyncEvent) GetRunnerID() string {
	return a.RunnerID
}

type LogLine struct {
	ID            string `json:"id"`
	ContainerName string `json:"containerName"`
	Message       string `json:"message"`
}

type LogEvent struct {
	AsyncEvent
	Lines []LogLine `json:"lines"`
}

type PingEvent struct {
	AsyncEvent
	Message string `json:"message"`
}

type NoticeEvent struct {
	AsyncEvent
	Severity string `json:"severity"`
	Message  string `json:"message"`
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
	return "", aet.scanner.Err()
}

func (aet *SseAsyncEventTransport) Close() error {
	return aet.httpResponse.Body.Close()
}

type AsyncEventManagerI interface {
	ParseEvent(event string) (AsyncEventI, error)
}

type AsyncEventManager struct {
	schema *jsonschema.Schema
}

func NewAsyncEventManager() (*AsyncEventManager, error) {
	schema, err := jsonschema.CompileString("schema.json", SCHEMA)
	if err != nil {
		return nil, err
	}

	asyncEventManager := AsyncEventManager{
		schema: schema,
	}

	return &asyncEventManager, nil
}

func (a *AsyncEventManager) ParseEvent(event string) (AsyncEventI, error) {
	err := a.schema.Validate(event)
	if err != nil {
		return nil, err
	}
	v := AsyncEvent{}
	if err := json.Unmarshal([]byte(event), &v); err != nil {
		log.Fatal(err)
	}

	if v.GetKind() == LOG {
		logEvent := LogEvent{}
		if err := json.Unmarshal([]byte(event), &logEvent); err != nil {
			log.Fatal(err)
		}
		return &logEvent, nil
	} else if v.GetKind() == NOTICE {
		noticeEvent := NoticeEvent{}
		if err := json.Unmarshal([]byte(event), &noticeEvent); err != nil {
			log.Fatal(err)
		}
		return &noticeEvent, nil
	} else if v.GetKind() == PING {
		pingEvent := PingEvent{}
		if err := json.Unmarshal([]byte(event), &pingEvent); err != nil {
			log.Fatal(err)
		}
		return &pingEvent, nil
	}

	return nil, fmt.Errorf("unknown event type: %s", v.GetKind())
}
