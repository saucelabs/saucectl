package imagerunner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogEvent(t *testing.T) {
	manager, err := NewAsyncEventManager()
	assert.NoError(t, err)

	eventMsg := `{
		"kind": "log",
		"runnerID": "myrunner",
		"lines": [
			{
				"id": "1",
				"containerName": "mycontainer",
				"message": "hello"
			}
		]
	}`

	event, err := manager.ParseEvent(eventMsg)
	assert.NoError(t, err)
	assert.Equal(t, "log", event.GetKind())
	assert.Equal(t, "myrunner", event.GetRunnerID())

	logEvent, ok := event.(*LogEvent)
	assert.True(t, ok)
	assert.Len(t, logEvent.Lines, 1)
	assert.Equal(t, "1", logEvent.Lines[0].ID)
	assert.Equal(t, "hello", logEvent.Lines[0].Message)
	assert.Equal(t, "mycontainer", logEvent.Lines[0].ContainerName)
}

func TestNoticeEvent(t *testing.T) {
	manager, err := NewAsyncEventManager()
	assert.NoError(t, err)

	eventMsg := `{
		"kind": "notice",
		"runnerID": "myrunner",
		"severity": "info",
		"message": "hello"
	}`

	event, err := manager.ParseEvent(eventMsg)
	assert.NoError(t, err)
	assert.Equal(t, "notice", event.GetKind())
	assert.Equal(t, "myrunner", event.GetRunnerID())

	noticeEvent, ok := event.(*NoticeEvent)
	assert.True(t, ok)
	assert.Equal(t, "notice", noticeEvent.GetKind())
	assert.Equal(t, "hello", noticeEvent.Message)
	assert.Equal(t, "info", noticeEvent.Severity)
}

func TestPingEvent(t *testing.T) {
	manager, err := NewAsyncEventManager()
	assert.NoError(t, err)

	eventMsg := `{
		"kind": "ping",
		"runnerID": "myrunner",
		"message": "hello"
	}`

	event, err := manager.ParseEvent(eventMsg)
	assert.NoError(t, err)
	assert.Equal(t, "ping", event.GetKind())
	assert.Equal(t, "myrunner", event.GetRunnerID())

	pingEvent, ok := event.(*PingEvent)
	assert.True(t, ok)
	assert.Equal(t, "hello", pingEvent.Message)
}
