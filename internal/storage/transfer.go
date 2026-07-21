package storage

import (
	"context"
	"fmt"

	"github.com/EgorGapo/bank/internal/domain"
)

func (s *Postgres) Transfer(ctx context.Context, amount int64, transferID string, fromAccountID string, toAccountId string, idempotencyKey string) (*domain.Transfer, error) {
	ans := &domain.Transfer{}
	if _, err := s.db.Exec(ctx, queryInsertTransfer, transferID, idempotencyKey, fromAccountID, toAccountId, amount, domain.TypeTransfer); err != nil {
		if isPgError(err, pgFKViolation) {
			return nil, domain.ErrAccountNotFound
		}
		if isPgError(err, pgUniqueViolation) {
			err := s.db.QueryRow(ctx, querySelectTransferByIdempotencyKey, idempotencyKey).
				Scan(&ans.ID, &ans.IdempotencyKey, &ans.FromAccountID, &ans.ToAccountID, &ans.Amount,
					&ans.Status, &ans.Type, &ans.CreatedAt, &ans.CompletedAt)
			if err != nil {
				return nil, fmt.Errorf("transfer: %w", err)
			}
			if amount != ans.Amount || ans.Type != domain.TypeTransfer || fromAccountID != *ans.FromAccountID || toAccountId != *ans.ToAccountID {
				return nil, domain.ErrIdempotencyKeyReuse
			}
			return ans, nil
		}
		return nil, fmt.Errorf("transfer: %w", err)
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("transfer failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	var toBalance int64
	var fromBalance int64

	if toAccountId > fromAccountID {
		if fromBalance, err = s.debitAccount(ctx, tx, fromAccountID, amount); err != nil {
			if isPgError(err, pgCheckViolation) {
				_ = tx.Rollback(ctx)
				if _, err := s.db.Exec(ctx, queryCancelTransfer, domain.StatusFailed, transferID); err != nil {
					return nil, err
				}
				return nil, domain.ErrNotEnoughMoney
			}
			return nil, fmt.Errorf("transfer: %w", err)
		}
		if toBalance, err = s.creditAccount(ctx, tx, toAccountId, amount); err != nil {
			return nil, fmt.Errorf("transfer: %w", err)
		}
	} else {
		if toBalance, err = s.creditAccount(ctx, tx, toAccountId, amount); err != nil {
			return nil, fmt.Errorf("transfer: %w", err)
		}
		if fromBalance, err = s.debitAccount(ctx, tx, fromAccountID, amount); err != nil {
			if isPgError(err, pgCheckViolation) {
				_ = tx.Rollback(ctx)
				if _, err := s.db.Exec(ctx, queryCancelTransfer, domain.StatusFailed, transferID); err != nil {
					return nil, err
				}
				return nil, domain.ErrNotEnoughMoney
			}
			return nil, fmt.Errorf("transfer: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, queryInsertLedgerEntry, transferID, toAccountId, amount, toBalance); err != nil {
		return nil, fmt.Errorf("transfer: %w", err)
	}
	if _, err := tx.Exec(ctx, queryInsertLedgerEntry, transferID, fromAccountID, -amount, fromBalance); err != nil {
		return nil, fmt.Errorf("transfer: %w", err)
	}

	if err := tx.QueryRow(ctx, queryCompleteTransfer, domain.StatusCompleted, transferID).
		Scan(&ans.ID, &ans.IdempotencyKey, &ans.FromAccountID, &ans.ToAccountID, &ans.Amount, &ans.Status,
			&ans.Type, &ans.CreatedAt, &ans.CompletedAt); err != nil {
		return nil, fmt.Errorf("transfer: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("transfer: %w", err)
	}
	return ans, nil
}
