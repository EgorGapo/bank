package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EgorGapo/bank/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const queryInsertInOutbox = `
	INSERT INTO outbox (id, topic, key, payload) VALUES ($1, $2, $3, $4)`

type OperationEvent struct {
	EventID       string    `json:"event_id"`
	Type          string    `json:"type"`
	TransferID    string    `json:"transfer_id"`
	FromAccountID *string   `json:"from_account_id,omitempty"`
	ToAccountID   *string   `json:"to_account_id,omitempty"`
	Amount        int64     `json:"amount"`
	Status        string    `json:"status"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type OutboxEvent struct {
	ID      string
	Topic   string
	Key     string
	Payload []byte
}

func buildOutboxEvent(transfer domain.Transfer, accountID string) OutboxEvent {
	operation := OperationEvent{
		EventID:       uuid.NewString(),
		Type:          transfer.Type,
		TransferID:    transfer.ID,
		FromAccountID: transfer.FromAccountID,
		ToAccountID:   transfer.ToAccountID,
		Amount:        transfer.Amount,
		Status:        transfer.Status,
		OccurredAt:    time.Now(),
	}
	payload, _ := json.Marshal(operation)

	event := OutboxEvent{
		ID:      operation.EventID,
		Topic:   domain.TopicLedgerOperations,
		Key:     accountID,
		Payload: payload,
	}
	return event

}

func (s *Postgres) insertOutboxEvent(ctx context.Context, tx pgx.Tx, event OutboxEvent) error {
	_, err := tx.Exec(ctx, queryInsertInOutbox, event.ID, event.Topic, event.Key, event.Payload)
	if err != nil {
		return fmt.Errorf("insertOutboxEvent: %w", err)
	}
	return nil
}
