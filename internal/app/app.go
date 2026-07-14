package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/EgorGapo/bank/internal/config"
	"github.com/EgorGapo/bank/internal/http/controller"
	"github.com/EgorGapo/bank/internal/http/route"
	"github.com/EgorGapo/bank/internal/storage"
	"github.com/EgorGapo/bank/internal/usecases"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(cfg *config.Config, logger *slog.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// зависимости
	store := storage.NewPostgres(pool, logger)
	usecase := usecases.NewBank(store, logger)
	ctrl := controller.New(logger, usecase)

	r := chi.NewRouter()
	route.Setup(r, ctrl)

	// TODO: запустить http.Server с роутером r и graceful shutdown (пункт 1)
	return nil
}
