// Package notify provides desktop notification abstractions.
package notify

// Notifier abstracts desktop notification delivery.
// Implementations exist for Linux (notify-send/zenity) and Windows (MessageBox).
type Notifier interface {
	// Show displays a notification with the given title and message.
	Show(title, message string) error
}

// Noop is a Notifier that silently discards all notifications.
type Noop struct{}

func (Noop) Show(_, _ string) error { return nil }
