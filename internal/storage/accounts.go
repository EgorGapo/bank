package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Postgres) CreateAccount(ctx context.Context, acc *domain.Account) error {
	query := `INSERT INTO accounts (id, status, balance) VALUES ($1, $2, $3) RETURNING created_at, updated_at`
	if err := s.db.QueryRow(ctx, query, acc.ID, acc.Status, acc.Balance).Scan(&acc.CreatedAt, &acc.UpdatedAt); err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

func (s *Postgres) GetAccount(ctx context.Context, id string) (*domain.Account, error) {
	ans := &domain.Account{}
	query := `SELECT id, balance, status, created_at, updated_at FROM accounts WHERE id = $1`
	err := s.db.QueryRow(ctx, query, id).Scan(&ans.ID, &ans.Balance, &ans.Status, &ans.CreatedAt, &ans.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, fmt.Errorf("get account: %w", err)
	}
	return ans, nil
}
