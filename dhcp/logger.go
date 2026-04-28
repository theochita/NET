package dhcp

import (
	"log"
	"time"
)

// LogEntry is a single structured log event emitted to the frontend.
// PascalCase fields — no json tags — Wails serialises them as-is.
type LogEntry struct {
	Time    string
	Level   string
	Message string
}

// EventLogger writes structured log entries to a stdlib logger and optionally
// calls an emit func so the frontend can receive them via Wails events.
// When emit is nil the logger is silent to the frontend — safe for headless use.
type EventLogger struct {
	std  *log.Logger
	emit func(LogEntry)
}

// NewEventLogger creates an EventLogger. emit may be nil.
func NewEventLogger(std *log.Logger, emit func(LogEntry)) *EventLogger {
	return &EventLogger{std: std, emit: emit}
}

func (l *EventLogger) write(level, msg string) {
	l.std.Printf("[%s] %s", level, msg)
	if l.emit != nil {
		l.emit(LogEntry{Time: time.Now().Format("15:04:05"), Level: level, Message: msg})
	}
}

// Info logs an informational message.
func (l *EventLogger) Info(msg string) { l.write("INFO", msg) }

// Warn logs a warning message.
func (l *EventLogger) Warn(msg string) { l.write("WARN", msg) }

// Error logs an error message.
func (l *EventLogger) Error(msg string) { l.write("ERROR", msg) }

// Packet logs a DHCP packet event.
func (l *EventLogger) Packet(msg string) { l.write("PKT", msg) }
