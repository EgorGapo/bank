package storage

import (
	"context"
	"fmt"

	"github.com/EgorGapo/bank/internal/domain"
)

func (s *Postgres) Deposit(ctx context.Context, amount int64, transferID string, toAccountId string, idempotencyKey string) (*domain.Transfer, error) {
	ans := &domain.Transfer{}
	if _, err := s.db.Exec(ctx, queryInsertTransfer, transferID, idempotencyKey, nil, toAccountId, amount, domain.TypeDeposit); err != nil {
		if isPgError(err, pgFKViolation) {
			return nil, domain.ErrAccountNotFound
		}
		if isPgError(err, pgUniqueViolation) {
			err := s.db.QueryRow(ctx, querySelectTransferByIdempotencyKey, idempotencyKey).
				Scan(&ans.ID, &ans.IdempotencyKey, &ans.FromAccountID, &ans.ToAccountID, &ans.Amount,
					&ans.Status, &ans.Type, &ans.CreatedAt, &ans.CompletedAt)
			if err != nil {
				return nil, fmt.Errorf("deposit: %w", err)
			}
			if amount != ans.Amount || ans.Type != domain.TypeDeposit || toAccountId != *ans.ToAccountID {
				return nil, domain.ErrIdempotencyKeyReuse
			}
			return ans, nil
		}
		return nil, fmt.Errorf("deposit: %w", err)
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("deposit failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	var newBalance int64
	if newBalance, err = s.creditAccount(ctx, tx, toAccountId, amount); err != nil {
		return nil, fmt.Errorf("deposit: %w", err)
	}
	if _, err := tx.Exec(ctx, queryInsertLedgerEntry, transferID, toAccountId, amount, newBalance); err != nil {
		return nil, fmt.Errorf("deposit: %w", err)
	}
	ans, err = s.completeTransfer(ctx, tx, transferID)
	if err != nil {
		return nil, fmt.Errorf("deposit: %w", err)
	}

	eventTo := buildOutboxEvent(*ans, toAccountId)
	if err := s.insertOutboxEvent(ctx, tx, eventTo); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("deposit: %w", err)
	}
	return ans, nil
}
