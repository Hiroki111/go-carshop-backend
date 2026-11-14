package kafkapublisher

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const publishTimeout = 10 * time.Second

type Publisher struct {
	client *kgo.Client
	topic  string
}

// New creates a publisher for one topic. Creating the client doesn't connect
// to any broker; the connections are opened on the first Publish. So the
// service can start while Kafka is down, and the relay reports the failures
// until Kafka is back.
//
// The client's defaults are what the relay needs: the producer is idempotent
// (a retry can't duplicate or reorder messages within a partition) and waits
// for all in-sync replicas to acknowledge each message.
func New(brokers []string, topic string) (*Publisher, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID("order-service"),
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka client: %w", err)
	}

	return &Publisher{client: client, topic: topic}, nil
}

// Publish writes one message and returns once Kafka has acknowledged it. The
// key decides the partition, so messages with the same key keep their order.
//
// An error doesn't prove the message was not written: a timeout can happen
// after Kafka stored it. The relay sends it again in that case, and
// consumers drop the duplicate by its event ID.
func (p *Publisher) Publish(ctx context.Context, key string, value []byte) error {
	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()

	record := &kgo.Record{Topic: p.topic, Key: []byte(key), Value: value}
	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("publish to %q: %w", p.topic, err)
	}
	return nil
}

// Close releases the client's connections. Call it after the relay has
// stopped.
func (p *Publisher) Close() {
	p.client.Close()
}
