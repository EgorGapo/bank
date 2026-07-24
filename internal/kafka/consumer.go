package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Consumer struct {
	client *kgo.Client
}

func NewConsumer(brokers []string, group string, topics ...string) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),     // consumer group — оффсеты хранятся в Kafka по группе
		kgo.ConsumeTopics(topics...), // какие топики читать
		kgo.DisableAutoCommit(),      // коммитим ВРУЧНУЮ — после записи в БД
	)
	if err != nil {
		return nil, fmt.Errorf("new kafka consumer: %w", err)
	}
	return &Consumer{client: client}, nil
}

// Poll блокирует до появления записей или отмены ctx.
func (c *Consumer) Poll(ctx context.Context) ([]*kgo.Record, error) {
	fetches := c.client.PollFetches(ctx)
	if err := fetches.Err(); err != nil {
		return nil, err // в т.ч. context.Canceled при shutdown
	}
	return fetches.Records(), nil
}

// Commit фиксирует оффсеты обработанных записей.
func (c *Consumer) Commit(ctx context.Context, records ...*kgo.Record) error {
	return c.client.CommitRecords(ctx, records...)
}

func (c *Consumer) Close() {
	c.client.Close()
}
