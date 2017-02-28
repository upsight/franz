package franz

import (
	"reflect"
	"testing"
	"time"

	"github.com/upsight/stop"
	"gopkg.in/Shopify/sarama.v1"
)

type MockConsumer struct {
	mockLogger
	*stop.ChannelStopper
	msgs []sarama.ConsumerMessage
	t    *testing.T
}

func (m *MockConsumer) Consume(msg *sarama.ConsumerMessage, partOffsetMgr sarama.PartitionOffsetManager) error {
	m.msgs = append(m.msgs, *msg)
	if len(m.msgs) >= 2 {
		m.Stop()
	}
	return nil
}

func (m *MockConsumer) StartOffset(partOffsetMgr sarama.PartitionOffsetManager) (int64, error) {
	offset, metadata := partOffsetMgr.NextOffset()
	if offset != int64(122) {
		m.t.Errorf("offset = %d; expected 122", offset)
	}
	if metadata != "metadata" {
		m.t.Errorf("metadata = %q; expected \"metadata\"", metadata)
	}
	return offset + 1, nil
}

func TestConsume(t *testing.T) {
	// sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)

	b := sarama.NewMockBroker(t, 0)
	b.SetHandlerByMap(map[string]sarama.MockResponse{
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetBroker(b.Addr(), b.BrokerID()).
			SetLeader("topic", 0, b.BrokerID()),
		"ConsumerMetadataRequest": sarama.NewMockConsumerMetadataResponse(t).
			SetCoordinator("group", b),
		"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t).
			SetOffset("group", "topic", 0, 122, "metadata", sarama.ErrNoError),
		"OffsetRequest": sarama.NewMockOffsetResponse(t).
			SetOffset("topic", 0, sarama.OffsetNewest, 123).
			SetOffset("topic", 0, sarama.OffsetOldest, 0),
		"FetchRequest": sarama.NewMockFetchResponse(t, 1).
			SetMessage("topic", 0, 123, sarama.StringEncoder("test message")).
			SetMessage("topic", 0, 124, sarama.StringEncoder("test message")).
			SetHighWaterMark("topic", 0, 124),
	})

	c := &MockConsumer{ChannelStopper: stop.NewChannelStopper(), t: t}
	Consume(c, []string{b.Addr()}, "group", "topic", 0)

	select {
	case <-c.StoppedChannel():
		expectedMsgs := []sarama.ConsumerMessage{
			sarama.ConsumerMessage{
				Offset:    123,
				Partition: 0,
				Topic:     "topic",
				Value:     []byte("test message"),
			},
			sarama.ConsumerMessage{
				Offset:    124,
				Partition: 0,
				Topic:     "topic",
				Value:     []byte("test message"),
			},
		}
		if !reflect.DeepEqual(c.msgs, expectedMsgs) {
			t.Errorf("c.msgs = %#v; expected %#v", c.msgs, expectedMsgs)
		}
	case <-time.After(1 * time.Second):
		t.Error("Consume() did not finish")
	}

	expectedEvents := []mockLoggerEvent{
		{
			event: "start",
			err:   nil,
			data: map[string]string{
				"offset":    "123",
				"partition": "0",
				"topic":     "topic",
			},
		},
		{
			event: "stopping",
			err:   nil,
			data:  nil,
		},
		{
			event: "stopped",
			err:   nil,
			data:  nil,
		},
	}
	if !reflect.DeepEqual(c.mockLogger.events, expectedEvents) {
		t.Errorf("c.mockLogger.events = %#v; expected %#v", c.mockLogger.events, expectedEvents)
	}
}
