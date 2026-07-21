package storage

import (
	"context"
	"fmt"

	"github.com/EgorGapo/bank/internal/domain"
)

const queryLdgerHistory = `
	SELECT id, transfer_id, account_id, amount, balance_after, created_at
	FROM ledger_entries
	WHERE account_id = $1 AND id < $2
	ORDER BY id DESC
	LIMIT $3`

func (s *Postgres) GetHistory(ctx context.Context, accountID string, cursor int64, limit int64) ([]domain.LedgerEntry, error) {
	rows, err := s.db.Query(ctx, queryLdgerHistory, accountID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}
	defer rows.Close()

	var entries []domain.LedgerEntry
	for rows.Next() {
		var e domain.LedgerEntry
		if err := rows.Scan(&e.ID, &e.TransferID, &e.AccountID, &e.Amount, &e.BalanceAfter, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("get history: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}

	return entries, nil

}
