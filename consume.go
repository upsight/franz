package franz

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/upsight/stop"
	sarama "gopkg.in/Shopify/sarama.v1"
)

// Consumer is a service that reads messages from a topic and partition
// and performs some action on them.
//
// Consume should process an individual message. It should handle
// updating any persisted offsets for the consumer. If an unrecoverable error
// occurs, the error should be returned.
//
// StartOffset should return the offset at which the consumer should start
// reading. If an error is returned, the consumer won't read any messages.
type Consumer interface {
	Logger
	stop.Stopper
	Consume(msg *sarama.ConsumerMessage, partOffsetMgr sarama.PartitionOffsetManager) error
	StartOffset(partOffsetMgr sarama.PartitionOffsetManager) (int64, error)
}

// Consume reads messages from the given topic and partition logs them. It
// will call StartOffset on the consumer to get the offset it should start
// reading from, and will call ConsumeMessage for each message it receives.
// It will continue reading sequentially, until Stop is called, ConsumeMessage
// returns an error, or an internal error occurs.
func Consume(c Consumer, addrs []string, group, topic string, partition int32) error {
	defer c.Stopped()

	client, err := sarama.NewClient(addrs, nil)
	if err != nil {
		return errors.Wrap(err, "error creating client")
	}
	defer closeAndLog(c, client, "close_client")

	mgr, err := sarama.NewOffsetManagerFromClient(group, client)
	if err != nil {
		return errors.Wrap(err, "error creating offset manager")
	}
	defer closeAndLog(c, mgr, "close_offset_manager")

	partMgr, err := mgr.ManagePartition(topic, partition)
	if err != nil {
		return errors.Wrap(err, "error managing partition")
	}
	defer closeAndLog(c, partMgr, "close_partition_manager")

	consumer, err := sarama.NewConsumerFromClient(client)
	if err != nil {
		return errors.Wrap(err, "error creating consumer")
	}
	defer closeAndLog(c, consumer, "close_consumer")

	// Finally we get to the actual object we are going to use: a partition
	// consumer. We fetch the next offset from where we left off from the
	// partition offset manager, and start consuming the same partition within
	// the topic, starting from that offset.
	offset, err := c.StartOffset(partMgr)
	if err != nil {
		return errors.Wrap(err, "error getting start offset")
	}

	partConsumer, err := consumer.ConsumePartition(topic, partition, offset)
	if err != nil {
		return errors.Wrap(err, "error consuming partition")
	}
	defer closeAndLog(c, partConsumer, "close_partition_consumer")

	// Start a ticker to emit a metric for the consumer lag every second. This
	// metric tells how many messages behind we are for within the partition.
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	go func() {
		for _ = range ticker.C {
			c.Gauge("lag", float64(partConsumer.HighWaterMarkOffset()-offset))
		}
	}()

	data := map[string]string{
		"offset":    strconv.FormatInt(offset, 10),
		"topic":     topic,
		"partition": strconv.FormatInt(int64(partition), 10),
	}
	c.Event("start", nil, data)
	defer c.Event("stopped", nil, nil)
	for {
		// Read messages from the consumer channel, and mark each offset after
		// processing it. This persists that value up to Kafka for your consumer
		// group, so if the worker is restarted, it can resume where it left off.
		//
		// The second argument to the MarkOffset function is metadata, which is
		// an arbitrary (but relatively short) string that your consumer is
		// supposed to be able to use to reconstruct where it left off. Maybe it
		// points to a file on disk or something with some persisted state in it.
		select {
		case <-c.StopChannel():
			c.Event("stopping", nil, nil)
			return nil
		case msg, ok := <-partConsumer.Messages():
			if !ok {
				return nil
			}
			atomic.StoreInt64(&offset, msg.Offset)
			if err := c.Consume(msg, partMgr); err != nil {
				data["offset"] = strconv.FormatInt(offset, 10)
				c.Event("consume_error", err, data)
			}
		}
	}
}
