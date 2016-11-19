package franz

import (
	"io"
)

// Logger is used to emit data about consumers or producers.
//
// Gauge is used to emit a value that changes over time (such as the offset
// lag for a consumer behind the high water mark).
//
// Event is used to emit general events, with an optional error.
type Logger interface {
	Event(event string, err error, data map[string]string)
	Gauge(gauge string, value float64)
}

// NoopLogger is an empty implementation of Logger that does nothing.
type NoopLogger struct{}

// Event does nothing.
func (n *NoopLogger) Event(event string, err error, data map[string]string) {}

// Gauge does nothing.
func (n *NoopLogger) Gauge(gauge string, value float64) {}

func closeAndLog(logger Logger, closer io.Closer, event string) error {
	if closer == nil {
		return nil
	}
	if err := closer.Close(); err != nil {
		if logger != nil {
			logger.Event(event, err, nil)
		}
		return err
	}
	return nil
}
