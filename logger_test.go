package franz

import (
	"errors"
	"io"
	"reflect"
	"testing"
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
	tests := []struct {
		err error
		mc  *mockCloser
		me  []mockLoggerEvent
	}{
		{},
		{
			mc: &mockCloser{},
		},
		{
			err: errors.New("test error"),
			mc:  &mockCloser{},
			me: []mockLoggerEvent{
				mockLoggerEvent{
					event: "test_event",
					err:   errors.New("test error"),
					data:  nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Logf("testing: %+v", tt)
		ml := &mockLogger{}
		var closer io.Closer
		if tt.mc != nil {
			closer = tt.mc
			tt.mc.err = tt.err
		}
		err := closeAndLog(ml, closer, "test_event")
		if err != tt.err {
			t.Errorf("err = %v; expected %v", err, tt.err)
		}
		if tt.mc != nil && tt.mc.closed != 1 {
			t.Errorf("tt.mc.closed = %d; expected 1", tt.mc.closed)
		}
		if !reflect.DeepEqual(ml.events, tt.me) {
			t.Errorf("ml.events = %#v; expected %#v", ml.events, tt.me)
		}
	}
}
