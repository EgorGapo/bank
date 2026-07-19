package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func isPgError(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func isPgConstraintViolation(err error, code string, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code && pgErr.ConstraintName == constraint
}

const queryInsertTransfer = `
	INSERT INTO transfers (id, idempotency_key, from_account_id, to_account_id, amount, type)
	VALUES ($1, $2, $3, $4, $5, $6)`

const querySelectTransferByIdempotencyKey = `
	SELECT id, idempotency_key, from_account_id, to_account_id, amount, status, type, created_at, completed_at
	FROM transfers
	WHERE idempotency_key = $1`

const queryAddToBalance = `
	UPDATE accounts
	SET balance = balance + $1
	WHERE id = $2
	RETURNING balance`

const querySubFromBalance = `
	UPDATE accounts
	SET balance = balance - $1
	WHERE id = $2
	RETURNING balance`

const queryInsertLedgerEntry = `
	INSERT INTO ledger_entries (transfer_id, account_id, amount, balance_after)
	VALUES ($1, $2, $3, $4)`

const queryCompleteTransfer = `
	UPDATE transfers
	SET status = $1, completed_at = now()
	WHERE id = $2 AND status = 'pending'
	RETURNING id, idempotency_key, from_account_id, to_account_id, amount, status, type, created_at, completed_at`

const queryCancelTransfer = `
	UPDATE transfers
	SET status = $1, completed_at = now()
	WHERE id = $2  AND status = 'pending'`

func (s *Postgres) debitAccount(ctx context.Context, tx pgx.Tx, accountID string, amount int64) (int64, error) {
	var newBalance int64
	if err := tx.QueryRow(ctx, querySubFromBalance, amount, accountID).Scan(&newBalance); err != nil {
		return 0, err
	}
	return newBalance, nil

}
func (s *Postgres) creditAccount(ctx context.Context, tx pgx.Tx, accountID string, amount int64) (int64, error) {
	var newBalance int64
	if err := tx.QueryRow(ctx, queryAddToBalance, amount, accountID).Scan(&newBalance); err != nil {
		return 0, err
	}
	return newBalance, nil
}

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
	if err := tx.QueryRow(ctx, queryCompleteTransfer, domain.StatusCompleted, transferID).
		Scan(&ans.ID, &ans.IdempotencyKey, &ans.FromAccountID, &ans.ToAccountID, &ans.Amount, &ans.Status,
			&ans.Type, &ans.CreatedAt, &ans.CompletedAt); err != nil {
		return nil, fmt.Errorf("deposit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("deposit: %w", err)
	}
	return ans, nil
}

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
			if _, err := s.db.Exec(ctx, queryCancelTransfer, domain.StatusFailed, transferID); err != nil {
				return nil, err
			}
			return nil, domain.ErrNotEnoughMoney
		}
		return nil, fmt.Errorf("withdraw: %w", err)
	}
	if _, err := tx.Exec(ctx, queryInsertLedgerEntry, transferID, fromAccountID, -amount, newBalance); err != nil {
		return nil, fmt.Errorf("withdraw: %w", err)
	}
	if err := tx.QueryRow(ctx, queryCompleteTransfer, domain.StatusCompleted, transferID).
		Scan(&ans.ID, &ans.IdempotencyKey, &ans.FromAccountID, &ans.ToAccountID, &ans.Amount, &ans.Status,
			&ans.Type, &ans.CreatedAt, &ans.CompletedAt); err != nil {
		return nil, fmt.Errorf("withdraw: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("withdraw: %w", err)
	}
	return ans, nil
}

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
