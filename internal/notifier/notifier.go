package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/EgorGapo/bank/internal/config"
	"github.com/EgorGapo/bank/internal/domain"
	"github.com/EgorGapo/bank/internal/kafka"
	"github.com/EgorGapo/bank/internal/storage"
	"github.com/EgorGapo/bank/internal/usecases"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Usecase interface {
	InsertNotification(ctx context.Context, eventID, accountID, text string) error
}

type Notifier struct {
	consumer *kafka.Consumer
	usecase  Usecase
	logger   *slog.Logger
}

func Run(logger *slog.Logger, cfg *config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poolCfg, err := pgxpool.ParseConfig(cfg.Postgres.DSN())
	if err != nil {
		return fmt.Errorf("parse pool config: %w", err)
	}
	poolCfg.MaxConns = cfg.Postgres.MaxConn

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	store := storage.NewPostgres(pool, logger)
	usecase := usecases.NewNotifier(store)

	consumer, err := kafka.NewConsumer(cfg.Kafka.Brokers, consumerGroup, domain.TopicLedgerOperations)
	if err != nil {
		return fmt.Errorf("create kafka consumer: %w", err)
	}
	defer consumer.Close()

	n := &Notifier{consumer: consumer, usecase: usecase, logger: logger}
	logger.Info("notifier started", "group", consumerGroup, "topic", domain.TopicLedgerOperations)
	return n.consumeLoop(ctx)
}
