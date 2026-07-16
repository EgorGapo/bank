package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

func isPgError(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func (s *Postgres) Deposit(ctx context.Context, amount int64, transferID string, accountID string, idempotencyKey string) (*domain.Transfer, error) {
	ans := &domain.Transfer{}
	queryTransfer := `INSERT INTO transfers (id, idempotency_key, to_account_id, amount, type) VALUES ($1,$2,$3,$4,$5)`
	if _, err := s.db.Exec(ctx, queryTransfer, transferID, idempotencyKey, accountID, amount, domain.TypeDeposit); err != nil {
		if isPgError(err, pgFKViolation) {
			return nil, domain.ErrAccountNotFound
		}
		if isPgError(err, pgUniqueViolation) {
			queryExistingTransfer := `SELECT id, idempotency_key, to_account_id,
											 amount, status, type, created_at, completed_at
									  FROM transfers WHERE idempotency_key = $1`
			err2 := s.db.QueryRow(ctx, queryExistingTransfer, idempotencyKey).
				Scan(&ans.ID, &ans.IdempotencyKey, &ans.ToAccountID, &ans.Amount,
					&ans.Status, &ans.Type, &ans.ErrCode, &ans.CreatedAt, &ans.CompletedAt)
			if err2 != nil {
				return nil, fmt.Errorf("deposit: %w", err2)
			}
			if amount != ans.Amount || accountID != ans.ToAccountID {
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
	queryUpdateBalance := `UPDATE accounts SET balance = balance + $1 WHERE id = $2 RETURNING balance`
	newBalance := 0
	if err := tx.QueryRow(ctx, queryUpdateBalance, amount, accountID).Scan(&newBalance); err != nil {
		return nil, fmt.Errorf("deposit: %w", err)
	}
	queryInsertIntoLedger := `INSERT INTO ledger_entries (transfer_id, account_id, amount, balance_after) VALUES ($1,$2,$3,$4)`
	if _, err := tx.Exec(ctx, queryInsertIntoLedger, transferID, accountID, amount, newBalance); err != nil {
		return nil, fmt.Errorf("deposit: %w", err)
	}
	queryUpdateTransferStatus := `UPDATE transfers SET status = $1, completed_at = now() WHERE id = $2 RETURNING id, 
								idempotency_key, to_account_id, amount, status, 
								type, created_at, completed_at`
	if err := tx.QueryRow(ctx, queryUpdateTransferStatus, domain.StatusCompleted, transferID).
		Scan(&ans.ID, &ans.IdempotencyKey, &ans.ToAccountID, &ans.Amount, &ans.Status,
			&ans.Type, &ans.CreatedAt, &ans.CompletedAt); err != nil {
		return nil, fmt.Errorf("deposit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("deposit: %w", err)
	}
	return ans, nil
}
