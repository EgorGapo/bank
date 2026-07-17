package storage

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

const pgFKViolation = "23503"
const pgUniqueViolation = "23505"
const pgCheckViolation = "23514"

type Postgres struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

func NewPostgres(db *pgxpool.Pool, logger *slog.Logger) *Postgres {
	return &Postgres{
		db:     db,
		logger: logger,
	}
}
