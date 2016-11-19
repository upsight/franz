package franz

import (
	"github.com/upsight/stop"
)

// Producer is a service that writes messages to a topic.
type Producer interface {
	Logger
	stop.Stopper

	// Produce should connect to the Kafka cluster using the given broker
	// addresses and write messages to the given topic. This should block until
	// it is done writing.
	Produce(addrs []string, topic string) error
}
