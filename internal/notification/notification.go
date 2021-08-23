package notification

// Notifier represents common interface for sending notifications.
type Notifier interface {
	SendMessage()
}
