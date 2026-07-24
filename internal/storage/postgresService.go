package storage

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type postgres struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

func NewPostgres(db *pgxpool.Pool, logger *slog.Logger) *postgres {
	return &postgres{
		db:     db,
		logger: logger,
	}
}
