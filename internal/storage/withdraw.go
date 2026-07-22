package storage

import (
	"context"
	"fmt"

	"github.com/EgorGapo/bank/internal/domain"
)

func (s *Postgres) Withdraw(ctx context.Context, amount int64, transferID string, fromAccountID string, idempotencyKey string) (*domain.Transfer, error) {
	ans := &domain.Transfer{}
	if _, err := s.db.Exec(ctx, queryInsertTransfer, transferID, idempotencyKey, fromAccountID, nil, amount, domain.TypeWithdraw); err != nil {
		if isPgError(err, pgFKViolation) {
			return nil, domain.ErrAccountNotFound
		}
		if isPgError(err, pgUniqueViolation) {
			err := s.db.QueryRow(ctx, querySelectTransferByIdempotencyKey, idempotencyKey).
				Scan(&ans.ID, &ans.IdempotencyKey, &ans.FromAccountID, &ans.ToAccountID, &ans.Amount,
					&ans.Status, &ans.Type, &ans.CreatedAt, &ans.CompletedAt)
			if err != nil {
				return nil, fmt.Errorf("withdraw: %w", err)
			}
			if amount != ans.Amount || ans.Type != domain.TypeWithdraw || fromAccountID != *ans.FromAccountID {
				return nil, domain.ErrIdempotencyKeyReuse
			}
			return ans, nil
		}
		return nil, fmt.Errorf("withdraw: %w", err)
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("withdraw failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	var newBalance int64
	if newBalance, err = s.debitAccount(ctx, tx, fromAccountID, amount); err != nil {
		if isPgError(err, pgCheckViolation) {
			_ = tx.Rollback(ctx)
			if err := s.markTransferFailed(ctx, transferID, domain.ErrCodeInsufficientFunds); err != nil {
				return nil, err
			}
			return nil, domain.ErrNotEnoughMoney

		}
		return nil, fmt.Errorf("withdraw: %w", err)
	}
	if _, err := tx.Exec(ctx, queryInsertLedgerEntry, transferID, fromAccountID, -amount, newBalance); err != nil {
		return nil, fmt.Errorf("withdraw: %w", err)
	}
	ans, err = s.completeTransfer(ctx, tx, transferID)
	if err != nil {
		return nil, fmt.Errorf("withdraw: %w", err)
	}

	eventFrom := buildOutboxEvent(*ans, fromAccountID)
	if err := s.insertOutboxEvent(ctx, tx, eventFrom); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("withdraw: %w", err)
	}
	return ans, nil
}
