package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/jackc/pgx/v5"
)

const queryInsertAccount = `
	INSERT INTO accounts (id, status, balance)
	VALUES ($1, $2, $3)
	RETURNING created_at, updated_at`

const querySelectAccount = `
	SELECT id, balance, status, created_at, updated_at
	FROM accounts
	WHERE id = $1`

func (s *postgres) CreateAccount(ctx context.Context, acc *domain.Account) error {
	if err := s.db.QueryRow(ctx, queryInsertAccount, acc.ID, acc.Status, acc.Balance).Scan(&acc.CreatedAt, &acc.UpdatedAt); err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	return nil
}

func (s *postgres) GetAccount(ctx context.Context, id string) (*domain.Account, error) {
	ans := &domain.Account{}
	err := s.db.QueryRow(ctx, querySelectAccount, id).Scan(&ans.ID, &ans.Balance, &ans.Status, &ans.CreatedAt, &ans.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, fmt.Errorf("get account: %w", err)
	}
	return ans, nil
}
