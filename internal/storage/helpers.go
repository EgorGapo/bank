package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Postgres) creditAccount(ctx context.Context, tx pgx.Tx, accountID string, amount int64) (int64, error) {
	var newBalance int64
	if err := tx.QueryRow(ctx, queryAddToBalance, amount, accountID).Scan(&newBalance); err != nil {
		return 0, err
	}
	return newBalance, nil
}
func (s *Postgres) debitAccount(ctx context.Context, tx pgx.Tx, accountID string, amount int64) (int64, error) {
	var newBalance int64
	if err := tx.QueryRow(ctx, querySubFromBalance, amount, accountID).Scan(&newBalance); err != nil {
		return 0, err
	}
	return newBalance, nil

}

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
