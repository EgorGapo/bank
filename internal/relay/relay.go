package relay

import (
	"context"
	"log/slog"
	"time"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/EgorGapo/bank/internal/kafka"
	"github.com/jackc/pgx/v5"
)

type Storage interface {
	FetchUnsentOutbox(ctx context.Context, tx pgx.Tx, limit int) ([]domain.OutboxEvent, error)
	MarkOutboxSent(ctx context.Context, tx pgx.Tx, ids []string) error
	WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type Relay struct {
	producer *kafka.Producer
	storage  Storage
	logger   *slog.Logger
}

func NewRelay(p *kafka.Producer, s Storage, logger *slog.Logger) *Relay {
	return &Relay{
		producer: p,
		storage:  s,
		logger:   logger,
	}
}

func (r *Relay) StartRelayEvent(ctx context.Context, workers int, tickerPeriod time.Duration) {
	for range workers {
		go r.worker(ctx, tickerPeriod)
	}

}

func (r *Relay) worker(ctx context.Context, period time.Duration) {
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("stopping event processing")
			return
		case <-ticker.C:
		}
		err := r.storage.WithTx(ctx, func(tx pgx.Tx) error {
			events, err := r.storage.FetchUnsentOutbox(ctx, tx, 10)
			if err != nil {
				return err
			}
			var ids []string
			for _, e := range events {
				if err := r.producer.Produce(ctx, e.Topic, []byte(e.Key), e.Payload); err != nil {
					r.logger.Warn("produce failed", "event_id", e.ID, "error", err)
					continue
				}
				ids = append(ids, e.ID)
			}
			if err := r.storage.MarkOutboxSent(ctx, tx, ids); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			r.logger.Error("relay batch failed", "error", err)
		}
	}

}
