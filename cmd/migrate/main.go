package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/EgorGapo/bank/internal/config"
	"github.com/EgorGapo/bank/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("migrate failed", "error", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")
}

func run(logger *slog.Logger) error {
	cfg, err := config.New()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer pool.Close()

	return db.SetupPostgres(pool)
}
