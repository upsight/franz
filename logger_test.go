package franz

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockCloser struct {
	closed int
	err    error
}

func (m *mockCloser) Close() error {
	m.closed++
	return m.err
}

type mockLoggerEvent struct {
	event string
	err   error
	data  map[string]string
}

type mockLoggerGauge struct {
	gauge string
	value float64
}

type mockLogger struct {
	events []mockLoggerEvent
	gauges []mockLoggerGauge
}

func (m *mockLogger) Event(event string, err error, data map[string]string) {
	m.events = append(m.events, mockLoggerEvent{
		event: event,
		err:   err,
		data:  data,
	})
}

func (m *mockLogger) Gauge(gauge string, value float64) {
	m.gauges = append(m.gauges, mockLoggerGauge{
		gauge: gauge,
		value: value,
	})
}

func TestCloseAndLog(t *testing.T) {
	ml := &mockLogger{}
	if err := closeAndLog(ml, nil, ""); assert.NoError(t, err) {
		assert.Empty(t, ml.events)
	}

	mc := &mockCloser{}
	if err := closeAndLog(ml, mc, ""); assert.NoError(t, err) {
		assert.Equal(t, 1, mc.closed)
		assert.Empty(t, ml.events)
	}

	mc = &mockCloser{err: errors.New("test error")}
	if err := closeAndLog(ml, mc, "test_event"); assert.Error(t, err) {
		assert.Equal(t, mc.err, err)
		assert.Equal(t, 1, mc.closed)
		assert.Equal(t, []mockLoggerEvent{
			{
				event: "test_event",
				err:   mc.err,
				data:  nil,
			},
		}, ml.events)
	}

	mc = &mockCloser{err: errors.New("test error")}
	if err := closeAndLog(nil, mc, "test_event"); assert.Error(t, err) {
		assert.Equal(t, mc.err, err)
		assert.Equal(t, 1, mc.closed)
	}
}
