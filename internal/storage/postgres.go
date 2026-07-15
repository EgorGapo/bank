package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

func (s *Postgres) CreateAccount(ctx context.Context, acc *domain.Account) error {
	query := `INSERT INTO accounts (id, status, balance) VALUES ($1, $2, $3) RETURNING created_at, updated_at`
	if err := s.db.QueryRow(ctx, query, acc.ID, acc.Status, acc.Balance).Scan(&acc.CreatedAt, &acc.UpdatedAt); err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}
