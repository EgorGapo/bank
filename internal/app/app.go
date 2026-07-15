package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/EgorGapo/bank/internal/config"
	"github.com/EgorGapo/bank/internal/http/controller"
	"github.com/EgorGapo/bank/internal/http/route"
	"github.com/EgorGapo/bank/internal/storage"
	"github.com/EgorGapo/bank/internal/usecases"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(cfg *config.Config, logger *slog.Logger) error {
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

	// зависимости
	store := storage.NewPostgres(pool, logger)
	usecase := usecases.NewBank(store, logger)
	ctrl := controller.New(logger, usecase)

	r := chi.NewRouter()
	route.Setup(r, ctrl, logger)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case err := <-errCh:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx) // ждём in-flight запросы
	}
}
