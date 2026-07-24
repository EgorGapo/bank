package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const queryInsertInOutbox = `
	INSERT INTO outbox (id, topic, key, payload) VALUES ($1, $2, $3, $4)`

const queryFetchUnsentEvents = `
	SELECT id, topic, key, payload, created_at
	FROM outbox
	WHERE sent_at IS NULL
	ORDER BY created_at
	LIMIT $1
	FOR UPDATE SKIP LOCKED
	`

const queryMarkSent = `
	UPDATE outbox SET sent_at = now() WHERE id = ANY($1)
	`

func buildOutboxEvent(transfer domain.Transfer, accountID string) domain.OutboxEvent {
	operation := domain.OperationEvent{
		EventID:       uuid.NewString(),
		Type:          transfer.Type,
		TransferID:    transfer.ID,
		FromAccountID: transfer.FromAccountID,
		ToAccountID:   transfer.ToAccountID,
		Amount:        transfer.Amount,
		Status:        transfer.Status,
		OccurredAt:    *transfer.CompletedAt,
	}
	payload, _ := json.Marshal(operation)

	event := domain.OutboxEvent{
		ID:      operation.EventID,
		Topic:   domain.TopicLedgerOperations,
		Key:     accountID,
		Payload: payload,
	}
	return event

}

func (s *postgres) insertOutboxEvent(ctx context.Context, tx pgx.Tx, event domain.OutboxEvent) error {
	_, err := tx.Exec(ctx, queryInsertInOutbox, event.ID, event.Topic, event.Key, event.Payload)
	if err != nil {
		return fmt.Errorf("insertOutboxEvent: %w", err)
	}
	return nil
}

func (s *postgres) FetchUnsentOutbox(ctx context.Context, tx pgx.Tx, limit int) ([]domain.OutboxEvent, error) {
	rows, err := tx.Query(ctx, queryFetchUnsentEvents, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch unsent outbox: %w", err)
	}
	defer rows.Close()

	var entries []domain.OutboxEvent
	for rows.Next() {
		var e domain.OutboxEvent
		if err := rows.Scan(&e.ID, &e.Topic, &e.Key, &e.Payload, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("fetch unsent outbox: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fetch unsent outbox: %w", err)
	}

	return entries, nil
}

func (s *postgres) MarkOutboxSent(ctx context.Context, tx pgx.Tx, ids []string) error {
	if _, err := tx.Exec(ctx, queryMarkSent, ids); err != nil {
		return fmt.Errorf("mark outbox sent: %w", err)
	}
	return nil
}
