package notifier

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/EgorGapo/bank/internal/domain"
)

func (n *Notifier) process(ctx context.Context, key, value []byte) error {
	event, err := parse(value)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	text, err := buildText(string(key), event)
	if err != nil {
		return fmt.Errorf("build text: %w", err)
	}
	if err := n.usecase.InsertNotification(ctx, event.EventID, string(key), text); err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

func parse(payload []byte) (*domain.OperationEvent, error) {
	res := &domain.OperationEvent{}
	if err := json.Unmarshal(payload, res); err != nil {
		return nil, err
	}
	return res, nil
}

func buildText(key string, event *domain.OperationEvent) (string, error) {
	switch event.Type {
	case domain.TypeDeposit:
		return fmt.Sprintf("пополнение %d", event.Amount), nil
	case domain.TypeWithdraw:
		return fmt.Sprintf("снятие %d", event.Amount), nil
	case domain.TypeTransfer:
		// у transfer два события (по from и по to); текст зависит от того, чей это счёт.
		if event.FromAccountID != nil && key == *event.FromAccountID {
			return fmt.Sprintf("снятие %d", event.Amount), nil
		}
		return fmt.Sprintf("пополнение %d", event.Amount), nil
	}
	return "", ErrUnknownOperationType
}
