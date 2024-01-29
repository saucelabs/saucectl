package imagerunner

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
